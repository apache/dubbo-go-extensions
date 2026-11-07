/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package hystrix

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"

	hystrixgo "github.com/afex/hystrix-go/hystrix"
)

const (
	hystrixExtensionName = "hystrix"
	defaultCommandName   = "default"
)

var (
	_ extension.Config     = (*Config)(nil)
	_ extension.Rollbacker = (*Config)(nil)
	_ extension.Option     = configOption{}
)

var (
	hystrixConfigMu         sync.Mutex
	hystrixRuntimeCommands  = make(map[string]commandRuntimeState)
	hystrixRuntimeDefaults  = make(map[extension.Scope]runtimeDefaultState)
	hystrixConfigGeneration uint64
)

// Config stores a Hystrix configuration for each Dubbo service method.
// Command names are resource names in the form interface:group:version:method.
type Config struct {
	Commands map[string]CommandConfig `yaml:",remain"`

	defaults       CommandConfig
	rollbackStates map[extension.Scope]configRollbackState
}

type commandRollbackState struct {
	previousSettings    hystrixgo.CommandConfig
	hadPreviousSettings bool
	previousRuntime     commandRuntimeState
	hadPreviousRuntime  bool
	configuredWith      hystrixgo.CommandConfig
}

type configRollbackState struct {
	generation         uint64
	commands           map[string]commandRollbackState
	previousDefault    runtimeDefaultState
	hadPreviousDefault bool
}

type commandRuntimeState struct {
	generation  uint64
	fromDefault bool
	scope       extension.Scope
}

type runtimeDefaultState struct {
	command    hystrixgo.CommandConfig
	enabled    bool
	generation uint64
}

// CommandConfig configures one Hystrix command. Zero values use hystrix-go's
// defaults.
type CommandConfig struct {
	Timeout                int            `yaml:"timeout"`
	MaxConcurrentRequests  int            `yaml:"max-concurrent-requests"`
	RequestVolumeThreshold int            `yaml:"request-volume-threshold"`
	SleepWindow            int            `yaml:"sleep-window"`
	ErrorPercentThreshold  int            `yaml:"error-percent-threshold"`
	UnknownFields          map[string]any `yaml:",remain"`
}

func (c *Config) Prefix() string {
	return hystrixExtensionName
}

func (c *Config) New() extension.Config {
	return &Config{
		Commands:       make(map[string]CommandConfig),
		defaults:       defaultCommandConfig(),
		rollbackStates: make(map[extension.Scope]configRollbackState),
	}
}

func (c *Config) Init(scope extension.Scope) error {
	if _, err := filterNameForScope(scope); err != nil {
		return err
	}

	commandNames := make([]string, 0, len(c.Commands))
	for commandName := range c.Commands {
		commandNames = append(commandNames, commandName)
	}
	sort.Strings(commandNames)

	for _, commandName := range commandNames {
		if err := validateCommandName(commandName); err != nil {
			return err
		}
		commandConfig := c.Commands[commandName]
		if len(commandConfig.UnknownFields) > 0 {
			unknownFields := make([]string, 0, len(commandConfig.UnknownFields))
			for fieldName := range commandConfig.UnknownFields {
				unknownFields = append(unknownFields, fieldName)
			}
			sort.Strings(unknownFields)
			return fmt.Errorf("hystrix command %q: unknown field(s): %s",
				commandName, strings.Join(unknownFields, ", "))
		}
	}

	c.defaults = defaultCommandConfig()
	hasDefault := false
	if defaultCommand, exists := c.Commands[defaultCommandName]; exists {
		var err error
		c.defaults, err = c.defaultedCommand(defaultCommand)
		if err != nil {
			return fmt.Errorf("hystrix command %q: %w", defaultCommandName, err)
		}
		hasDefault = true
	}

	configured := make(map[string]CommandConfig, len(commandNames))
	for _, commandName := range commandNames {
		if commandName == defaultCommandName {
			continue
		}
		command, err := c.defaultedCommand(c.Commands[commandName])
		if err != nil {
			return fmt.Errorf("hystrix command %q: %w", commandName, err)
		}
		configured[internalCommandName(scope, commandName)] = command
	}

	hystrixConfigMu.Lock()
	defer hystrixConfigMu.Unlock()
	hystrixConfigGeneration++
	generation := hystrixConfigGeneration
	rollbackState := configRollbackState{
		generation: generation,
		commands:   make(map[string]commandRollbackState, len(configured)),
	}
	if previousDefault, exists := hystrixRuntimeDefaults[scope]; exists {
		rollbackState.previousDefault = previousDefault
		rollbackState.hadPreviousDefault = true
	}
	hystrixRuntimeDefaults[scope] = runtimeDefaultState{
		command:    toHystrixCommandConfig(c.defaults),
		enabled:    hasDefault,
		generation: generation,
	}
	existingSettings := hystrixgo.GetCircuitSettings()
	for _, commandName := range commandNames {
		if commandName == defaultCommandName {
			continue
		}
		internalName := internalCommandName(scope, commandName)
		command := configured[internalName]
		hystrixCommand := toHystrixCommandConfig(command)
		state := commandRollbackState{configuredWith: hystrixCommand}
		if previous, exists := existingSettings[internalName]; exists {
			state.previousSettings = commandConfigFromSettings(previous)
			state.hadPreviousSettings = true
		}
		if previous, exists := hystrixRuntimeCommands[internalName]; exists {
			state.previousRuntime = previous
			state.hadPreviousRuntime = true
		}
		rollbackState.commands[internalName] = state
		hystrixRuntimeCommands[internalName] = commandRuntimeState{generation: generation, scope: scope}
		hystrixgo.ConfigureCommand(internalName, hystrixCommand)
	}
	if c.rollbackStates == nil {
		c.rollbackStates = make(map[extension.Scope]configRollbackState)
	}
	c.rollbackStates[scope] = rollbackState
	return nil
}

// Rollback restores settings written by Init when a later extension fails
// during the same initialization attempt. Hystrix keeps settings process-wide,
// so clients or servers using the same command name also share its settings.
func (c *Config) Rollback(scope extension.Scope) error {
	hystrixConfigMu.Lock()
	defer hystrixConfigMu.Unlock()

	rollbackState, exists := c.rollbackStates[scope]
	if !exists {
		return nil
	}
	for commandName, state := range rollbackState.commands {
		currentRuntime, exists := hystrixRuntimeCommands[commandName]
		if !exists || currentRuntime.generation != rollbackState.generation {
			continue
		}
		current, exists := hystrixgo.GetCircuitSettings()[commandName]
		if !exists || !sameCommandConfig(commandConfigFromSettings(current), state.configuredWith) {
			continue
		}
		if state.hadPreviousSettings {
			hystrixgo.ConfigureCommand(commandName, state.previousSettings)
		} else {
			// hystrix-go has no per-command delete API; default settings have
			// the same behavior as an absent settings entry on the next call.
			hystrixgo.ConfigureCommand(commandName, hystrixgo.CommandConfig{})
		}
		if state.hadPreviousRuntime {
			hystrixRuntimeCommands[commandName] = state.previousRuntime
		} else {
			delete(hystrixRuntimeCommands, commandName)
		}
	}
	if currentDefault, exists := hystrixRuntimeDefaults[scope]; exists && currentDefault.generation == rollbackState.generation {
		if rollbackState.hadPreviousDefault {
			hystrixRuntimeDefaults[scope] = rollbackState.previousDefault
		} else {
			delete(hystrixRuntimeDefaults, scope)
		}
	}
	for commandName, commandState := range hystrixRuntimeCommands {
		if commandState.scope == scope && commandState.fromDefault && commandState.generation == rollbackState.generation {
			delete(hystrixRuntimeCommands, commandName)
		}
	}
	delete(c.rollbackStates, scope)
	return nil
}

func commandConfigFromSettings(settings *hystrixgo.Settings) hystrixgo.CommandConfig {
	return hystrixgo.CommandConfig{
		Timeout:                int(settings.Timeout.Milliseconds()),
		MaxConcurrentRequests:  settings.MaxConcurrentRequests,
		RequestVolumeThreshold: int(settings.RequestVolumeThreshold),
		SleepWindow:            int(settings.SleepWindow.Milliseconds()),
		ErrorPercentThreshold:  settings.ErrorPercentThreshold,
	}
}

func sameCommandConfig(left, right hystrixgo.CommandConfig) bool {
	return left.Timeout == right.Timeout &&
		left.MaxConcurrentRequests == right.MaxConcurrentRequests &&
		left.RequestVolumeThreshold == right.RequestVolumeThreshold &&
		left.SleepWindow == right.SleepWindow &&
		left.ErrorPercentThreshold == right.ErrorPercentThreshold
}

func toHystrixCommandConfig(command CommandConfig) hystrixgo.CommandConfig {
	return hystrixgo.CommandConfig{
		Timeout:                command.Timeout,
		MaxConcurrentRequests:  command.MaxConcurrentRequests,
		RequestVolumeThreshold: command.RequestVolumeThreshold,
		SleepWindow:            command.SleepWindow,
		ErrorPercentThreshold:  command.ErrorPercentThreshold,
	}
}

func ensureRuntimeCommand(scope extension.Scope, commandName string) bool {
	hystrixConfigMu.Lock()
	defer hystrixConfigMu.Unlock()

	registration, exists := hystrixRuntimeCommands[commandName]
	if exists && !registration.fromDefault {
		return true
	}
	defaults, hasDefault := hystrixRuntimeDefaults[scope]
	if !hasDefault || !defaults.enabled {
		if exists {
			delete(hystrixRuntimeCommands, commandName)
		}
		return false
	}
	if exists && registration.fromDefault && registration.generation == defaults.generation {
		return true
	}
	hystrixgo.ConfigureCommand(commandName, defaults.command)
	hystrixRuntimeCommands[commandName] = commandRuntimeState{
		generation:  defaults.generation,
		fromDefault: true,
		scope:       scope,
	}
	return true
}

func (c *Config) FilterNames(scope extension.Scope) []string {
	filterName, err := filterNameForScope(scope)
	if err != nil {
		// The core asks for filter names before Init; Init reports unsupported scopes.
		return nil
	}
	return []string{filterName}
}

func filterNameForScope(scope extension.Scope) (string, error) {
	switch scope {
	case extension.ClientScope:
		return constant.HystrixConsumerFilterKey, nil
	case extension.ServerScope:
		return constant.HystrixProviderFilterKey, nil
	default:
		return "", fmt.Errorf("hystrix does not support scope %d", scope)
	}
}

func defaultCommandConfig() CommandConfig {
	return CommandConfig{
		Timeout:                hystrixgo.DefaultTimeout,
		MaxConcurrentRequests:  hystrixgo.DefaultMaxConcurrent,
		RequestVolumeThreshold: hystrixgo.DefaultVolumeThreshold,
		SleepWindow:            hystrixgo.DefaultSleepWindow,
		ErrorPercentThreshold:  hystrixgo.DefaultErrorPercentThreshold,
	}
}

func validateCommandName(commandName string) error {
	if strings.TrimSpace(commandName) == "" {
		return fmt.Errorf("hystrix command name is required")
	}
	if strings.TrimSpace(commandName) != commandName {
		return fmt.Errorf("hystrix command name %q must not contain surrounding whitespace", commandName)
	}
	if commandName == defaultCommandName {
		return nil
	}
	if strings.Count(commandName, ":") != 3 {
		return fmt.Errorf("hystrix command %q must use interface:group:version:method format", commandName)
	}
	return nil
}

func isZeroCommandConfig(command CommandConfig) bool {
	return command.Timeout == 0 &&
		command.MaxConcurrentRequests == 0 &&
		command.RequestVolumeThreshold == 0 &&
		command.SleepWindow == 0 &&
		command.ErrorPercentThreshold == 0 &&
		len(command.UnknownFields) == 0
}

func (c *Config) defaultedCommand(command CommandConfig) (CommandConfig, error) {
	defaults := c.defaults
	if isZeroCommandConfig(defaults) {
		defaults = defaultCommandConfig()
	}
	fields := []*int{
		&command.Timeout,
		&command.MaxConcurrentRequests,
		&command.RequestVolumeThreshold,
		&command.SleepWindow,
		&command.ErrorPercentThreshold,
	}
	defaultFields := []int{
		defaults.Timeout,
		defaults.MaxConcurrentRequests,
		defaults.RequestVolumeThreshold,
		defaults.SleepWindow,
		defaults.ErrorPercentThreshold,
	}
	fieldNames := []string{
		"timeout",
		"max-concurrent-requests",
		"request-volume-threshold",
		"sleep-window",
		"error-percent-threshold",
	}
	for index, field := range fields {
		if *field < 0 {
			return CommandConfig{}, fmt.Errorf("%s must not be negative", fieldNames[index])
		}
		if *field == 0 {
			*field = defaultFields[index]
		}
	}
	if command.ErrorPercentThreshold > 100 {
		return CommandConfig{}, fmt.Errorf("error-percent-threshold must be at most 100")
	}
	return command, nil
}

// ConfigOption changes one command in a WithConfig option.
type ConfigOption func(*commandOptions)

type commandOptions struct {
	name   string
	values CommandConfig
	fields uint8
}

const (
	timeoutField uint8 = 1 << iota
	maxConcurrentRequestsField
	requestVolumeThresholdField
	sleepWindowField
	errorPercentThresholdField
)

// WithCommandName selects the service-method resource name configured by
// WithConfig. For example: "greet.GreetService:::Greet".
func WithCommandName(name string) ConfigOption {
	return func(options *commandOptions) { options.name = name }
}

// WithTimeout sets the command timeout in milliseconds.
func WithTimeout(timeout int) ConfigOption {
	return func(options *commandOptions) {
		options.values.Timeout = timeout
		options.fields |= timeoutField
	}
}

// WithMaxConcurrentRequests sets the maximum number of concurrent executions.
func WithMaxConcurrentRequests(max int) ConfigOption {
	return func(options *commandOptions) {
		options.values.MaxConcurrentRequests = max
		options.fields |= maxConcurrentRequestsField
	}
}

// WithRequestVolumeThreshold sets the minimum request count used to evaluate
// the circuit error percentage.
func WithRequestVolumeThreshold(threshold int) ConfigOption {
	return func(options *commandOptions) {
		options.values.RequestVolumeThreshold = threshold
		options.fields |= requestVolumeThresholdField
	}
}

// WithSleepWindow sets the time before Hystrix allows a trial request, in
// milliseconds.
func WithSleepWindow(window int) ConfigOption {
	return func(options *commandOptions) {
		options.values.SleepWindow = window
		options.fields |= sleepWindowField
	}
}

// WithErrorPercentThreshold sets the error percentage that opens the circuit.
func WithErrorPercentThreshold(threshold int) ConfigOption {
	return func(options *commandOptions) {
		options.values.ErrorPercentThreshold = threshold
		options.fields |= errorPercentThresholdField
	}
}

type configOption struct {
	options []ConfigOption
}

func (configOption) Prefix() string {
	return hystrixExtensionName
}

func (option configOption) Apply(config extension.Config) error {
	hystrixConfig, ok := config.(*Config)
	if !ok || hystrixConfig == nil {
		return fmt.Errorf("hystrix received unexpected config type %T", config)
	}
	if hystrixConfig.Commands == nil {
		hystrixConfig.Commands = make(map[string]CommandConfig)
	}

	options := commandOptions{}
	for index, apply := range option.options {
		if apply == nil {
			return fmt.Errorf("hystrix config option %d is nil", index)
		}
		apply(&options)
	}
	if strings.TrimSpace(options.name) == "" {
		return fmt.Errorf("hystrix command name is required")
	}
	if err := validateCommandName(options.name); err != nil {
		return err
	}

	command, exists := hystrixConfig.Commands[options.name]
	if !exists {
		command = CommandConfig{}
	}
	if options.fields&timeoutField != 0 {
		command.Timeout = options.values.Timeout
	}
	if options.fields&maxConcurrentRequestsField != 0 {
		command.MaxConcurrentRequests = options.values.MaxConcurrentRequests
	}
	if options.fields&requestVolumeThresholdField != 0 {
		command.RequestVolumeThreshold = options.values.RequestVolumeThreshold
	}
	if options.fields&sleepWindowField != 0 {
		command.SleepWindow = options.values.SleepWindow
	}
	if options.fields&errorPercentThresholdField != 0 {
		command.ErrorPercentThreshold = options.values.ErrorPercentThreshold
	}
	if _, err := hystrixConfig.defaultedCommand(command); err != nil {
		return fmt.Errorf("hystrix command %q: %w", options.name, err)
	}
	hystrixConfig.Commands[options.name] = command
	return nil
}

// WithConfig configures one command for a client or server. Use multiple
// WithConfig values to configure multiple command names.
func WithConfig(options ...ConfigOption) extension.Option {
	return configOption{options: append([]ConfigOption(nil), options...)}
}

func internalCommandName(scope extension.Scope, resourceName string) string {
	if scope == extension.ServerScope {
		return internalProviderCommandPrefix + resourceName
	}
	return internalConsumerCommandPrefix + resourceName
}
