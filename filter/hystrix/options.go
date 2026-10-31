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
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"

	hystrixgo "github.com/afex/hystrix-go/hystrix"
)

const hystrixExtensionName = "hystrix"

var (
	_ extension.Config = (*Config)(nil)
	_ extension.Option = configOption{}
)

// Config stores a Hystrix configuration for each Dubbo service method.
// Command names are resource names in the form interface:group:version:method.
type Config struct {
	Commands map[string]CommandConfig `yaml:",remain"`

	defaults CommandConfig
}

// CommandConfig configures one Hystrix command. Zero values use hystrix-go's
// defaults.
type CommandConfig struct {
	Timeout                int `yaml:"timeout"`
	MaxConcurrentRequests  int `yaml:"max-concurrent-requests"`
	RequestVolumeThreshold int `yaml:"request-volume-threshold"`
	SleepWindow            int `yaml:"sleep-window"`
	ErrorPercentThreshold  int `yaml:"error-percent-threshold"`
}

func (c *Config) Prefix() string {
	return hystrixExtensionName
}

func (c *Config) New() extension.Config {
	return &Config{
		Commands: make(map[string]CommandConfig),
		defaults: defaultCommandConfig(),
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

	configured := make(map[string]CommandConfig, len(commandNames))
	for _, commandName := range commandNames {
		if strings.TrimSpace(commandName) == "" {
			return fmt.Errorf("hystrix command name is required")
		}
		if strings.TrimSpace(commandName) != commandName {
			return fmt.Errorf("hystrix command name %q must not contain surrounding whitespace", commandName)
		}
		command, err := c.defaultedCommand(c.Commands[commandName])
		if err != nil {
			return fmt.Errorf("hystrix command %q: %w", commandName, err)
		}
		configured[internalCommandName(scope, commandName)] = command
	}

	for _, commandName := range commandNames {
		internalName := internalCommandName(scope, commandName)
		command := configured[internalName]
		hystrixgo.ConfigureCommand(internalName, hystrixgo.CommandConfig{
			Timeout:                command.Timeout,
			MaxConcurrentRequests:  command.MaxConcurrentRequests,
			RequestVolumeThreshold: command.RequestVolumeThreshold,
			SleepWindow:            command.SleepWindow,
			ErrorPercentThreshold:  command.ErrorPercentThreshold,
		})
	}
	return nil
}

func (c *Config) FilterNames(scope extension.Scope) []string {
	filterName, err := filterNameForScope(scope)
	if err != nil {
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

func (c *Config) defaultedCommand(command CommandConfig) (CommandConfig, error) {
	defaults := c.defaults
	if defaults == (CommandConfig{}) {
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
		if len(option.options) == 0 {
			return nil
		}
		return fmt.Errorf("hystrix command name is required")
	}
	if strings.TrimSpace(options.name) != options.name {
		return fmt.Errorf("hystrix command name %q must not contain surrounding whitespace", options.name)
	}

	command, exists := hystrixConfig.Commands[options.name]
	if !exists {
		command = hystrixConfig.defaults
		if command == (CommandConfig{}) {
			command = defaultCommandConfig()
		}
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
