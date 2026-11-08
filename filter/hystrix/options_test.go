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
	"context"
	"testing"
)

import (
	"dubbo.apache.org/dubbo-go/v3/client"
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/server"

	hystrixgo "github.com/afex/hystrix-go/hystrix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigRegistrationAndNew(t *testing.T) {
	prototype, ok := extension.LookupConfig(hystrixExtensionName)
	require.True(t, ok)
	assert.IsType(t, &Config{}, prototype)

	first := prototype.New().(*Config)
	second := prototype.New().(*Config)
	assert.NotSame(t, first, second)
	first.Commands["independent:::Run"] = CommandConfig{Timeout: 1}
	assert.Empty(t, second.Commands)
	assert.Equal(t, hystrixExtensionName, first.Prefix())
	assert.Equal(t, defaultCommandConfig(), first.defaults)
}

func TestDefaultCommandAppliesToUnconfiguredMethods(t *testing.T) {
	isolateRuntimeConfiguration(t)

	const (
		configuredResource = "hystrix-default-configured:::Run"
		unconfiguredMethod = "hystrix-default-unconfigured:::Run"
	)
	config := (&Config{}).New().(*Config)
	require.NoError(t, WithConfig(
		WithCommandName(defaultCommandName),
		WithTimeout(425),
		WithMaxConcurrentRequests(17),
	).Apply(config))
	require.NoError(t, WithConfig(
		WithCommandName(configuredResource),
		WithTimeout(600),
	).Apply(config))
	require.NoError(t, config.Init(extension.ClientScope))
	t.Cleanup(func() {
		assert.NoError(t, config.Rollback(extension.ClientScope))
	})

	settings := hystrixgo.GetCircuitSettings()
	configured, ok := settings[internalCommandName(extension.ClientScope, configuredResource)]
	require.True(t, ok)
	assert.Equal(t, 600, int(configured.Timeout.Milliseconds()))
	assert.Equal(t, 17, configured.MaxConcurrentRequests)
	_, defaultIsCommand := settings[internalCommandName(extension.ClientScope, defaultCommandName)]
	assert.False(t, defaultIsCommand)

	url, err := common.NewURL("dubbo://127.0.0.1:20000/hystrix-default-unconfigured")
	require.NoError(t, err)
	invoker := base.NewBaseInvoker(url)
	hystrixFilter := &Filter{isConsumer: true}
	invocation := invocation.NewRPCInvocation("Run", []any{}, make(map[string]any))
	invokeResult := hystrixFilter.Invoke(context.Background(), invoker, invocation)
	require.NoError(t, invokeResult.Error())
	defaulted, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ClientScope, unconfiguredMethod)]
	require.True(t, ok)
	assert.Equal(t, 425, int(defaulted.Timeout.Milliseconds()))
	assert.Equal(t, 17, defaulted.MaxConcurrentRequests)
}

func TestConfigScopeAndCommandDefaults(t *testing.T) {
	const resourceName = "hystrix-config-scope-test:::Run"
	config := (&Config{}).New().(*Config)
	config.Commands[resourceName] = CommandConfig{Timeout: 325}

	require.NoError(t, config.Init(extension.ClientScope))
	settings := hystrixgo.GetCircuitSettings()
	clientSettings, ok := settings[internalCommandName(extension.ClientScope, resourceName)]
	require.True(t, ok)
	assert.Equal(t, 325, int(clientSettings.Timeout.Milliseconds()))
	assert.Equal(t, defaultCommandConfig().MaxConcurrentRequests, clientSettings.MaxConcurrentRequests)
	assert.Equal(t, []string{constant.HystrixConsumerFilterKey}, config.FilterNames(extension.ClientScope))

	assert.Error(t, config.Init(extension.InstanceScope))
	assert.Nil(t, config.FilterNames(extension.InstanceScope))
}

func TestConfigRollbackRestoresPreviousSettings(t *testing.T) {
	const resourceName = "hystrix-rollback-test:::Run"
	internalName := internalCommandName(extension.ClientScope, resourceName)
	hystrixgo.ConfigureCommand(internalName, hystrixgo.CommandConfig{
		Timeout:                125,
		MaxConcurrentRequests:  11,
		RequestVolumeThreshold: 6,
		SleepWindow:            1500,
		ErrorPercentThreshold:  60,
	})

	config := (&Config{}).New().(*Config)
	config.Commands[resourceName] = CommandConfig{Timeout: 250}
	require.NoError(t, config.Init(extension.ClientScope))
	updatedSettings, ok := hystrixgo.GetCircuitSettings()[internalName]
	require.True(t, ok)
	assert.Equal(t, 250, int(updatedSettings.Timeout.Milliseconds()))

	require.NoError(t, config.Rollback(extension.ClientScope))
	settings := hystrixgo.GetCircuitSettings()[internalName]
	require.NotNil(t, settings)
	assert.Equal(t, 125, int(settings.Timeout.Milliseconds()))
	assert.Equal(t, 11, settings.MaxConcurrentRequests)
}

func TestConfigRollbackRestoresPreviousInitGeneration(t *testing.T) {
	isolateRuntimeConfiguration(t)

	const resourceName = "hystrix-rollback-generation-test:::Run"
	internalName := internalCommandName(extension.ClientScope, resourceName)
	config := (&Config{}).New().(*Config)
	config.Commands[resourceName] = CommandConfig{Timeout: 125}
	require.NoError(t, config.Init(extension.ClientScope))

	config.Commands[resourceName] = CommandConfig{Timeout: 250}
	require.NoError(t, config.Init(extension.ClientScope))
	assert.Equal(t, 250, int(hystrixgo.GetCircuitSettings()[internalName].Timeout.Milliseconds()))

	require.NoError(t, config.Rollback(extension.ClientScope))
	settings := hystrixgo.GetCircuitSettings()[internalName]
	require.NotNil(t, settings)
	assert.Equal(t, 125, int(settings.Timeout.Milliseconds()))
	assert.True(t, ensureRuntimeCommand(extension.ClientScope, internalName))
}

func TestInitializeYAMLAndTypedOptions(t *testing.T) {
	const (
		resourceName = "greet.GreetService:::Greet"
		secondName   = "sample.Service:group:v1:Other"
	)
	raw := map[string]any{
		hystrixExtensionName: map[string]any{
			"consumer": map[string]any{
				resourceName: map[string]any{
					"timeout":                  150,
					"max-concurrent-requests":  7,
					"request-volume-threshold": 3,
				},
				secondName: map[string]any{"timeout": 250},
			},
			"provider": map[string]any{
				resourceName: map[string]any{"timeout": 225},
			},
		},
	}
	options := []extension.Option{
		WithConfig(WithCommandName(resourceName), WithTimeout(350)),
		WithConfig(WithCommandName(resourceName), WithSleepWindow(2200)),
		WithConfig(WithCommandName(secondName), WithTimeout(450)),
	}

	clientFilters, err := extension.Initialize(raw, options, extension.ClientScope)
	require.NoError(t, err)
	assert.Equal(t, []string{constant.HystrixConsumerFilterKey}, clientFilters)
	assert.Equal(t, "existing,"+constant.HystrixConsumerFilterKey,
		extension.MergeFilterNames("existing", clientFilters))
	assert.Empty(t, extension.MergeFilterNames("-"+constant.HystrixConsumerFilterKey, clientFilters))
	settings := hystrixgo.GetCircuitSettings()
	clientCommand, ok := settings[internalCommandName(extension.ClientScope, resourceName)]
	require.True(t, ok)
	assert.Equal(t, 350, int(clientCommand.Timeout.Milliseconds()))
	assert.Equal(t, 7, clientCommand.MaxConcurrentRequests)
	assert.Equal(t, 3, int(clientCommand.RequestVolumeThreshold))
	assert.Equal(t, 2200, int(clientCommand.SleepWindow.Milliseconds()))

	secondCommand, ok := settings[internalCommandName(extension.ClientScope, secondName)]
	require.True(t, ok)
	assert.Equal(t, 450, int(secondCommand.Timeout.Milliseconds()))

	serverFilters, err := extension.Initialize(raw, nil, extension.ServerScope)
	require.NoError(t, err)
	assert.Equal(t, []string{constant.HystrixProviderFilterKey}, serverFilters)
	providerCommand, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ServerScope, resourceName)]
	require.True(t, ok)
	assert.Equal(t, 225, int(providerCommand.Timeout.Milliseconds()))
	assert.NotEqual(t, internalCommandName(extension.ClientScope, resourceName),
		internalCommandName(extension.ServerScope, resourceName))
}

func TestConfigRejectsInvalidValuesAndOptions(t *testing.T) {
	config := (&Config{}).New().(*Config)
	config.Commands["invalid:::Run"] = CommandConfig{Timeout: -1}
	require.Error(t, config.Init(extension.ClientScope))

	config = (&Config{}).New().(*Config)
	config.Commands["commands"] = CommandConfig{Timeout: 10}
	require.Error(t, config.Init(extension.ClientScope))

	var nilConfig *Config
	assert.Error(t, WithConfig().Apply(nilConfig))
	assert.Error(t, WithConfig().Apply(&Config{}))
	assert.Error(t, WithConfig(nil).Apply(&Config{}))
	assert.Error(t, WithConfig(WithTimeout(10)).Apply(&Config{}))
	assert.Error(t, WithConfig(WithCommandName("invalid:::Run"), WithTimeout(-1)).Apply(&Config{}))
}

func TestYAMLConfigRejectsUnknownCommandFields(t *testing.T) {
	_, err := extension.Initialize(map[string]any{
		hystrixExtensionName: map[string]any{
			"consumer": map[string]any{
				"greet.GreetService:::Greet": map[string]any{"timeoutt": 42},
			},
		},
	}, nil, extension.ClientScope)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeoutt")
}

func TestWithConfigIntegratesWithClientAndServer(t *testing.T) {
	cli, err := client.NewClient(client.WithExtension(
		WithConfig(WithCommandName("hystrix-client-integration:::Run")),
	))
	require.NoError(t, err)
	assert.NotNil(t, cli)

	srv, err := server.NewServer(server.WithExtension(
		WithConfig(WithCommandName("hystrix-server-integration:::Run")),
	))
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestYAMLConfigIntegratesWithClientAndServer(t *testing.T) {
	isolateRuntimeConfiguration(t)

	const resourceName = "hystrix-yaml-integration:::Run"
	configs := map[string]any{
		hystrixExtensionName: map[string]any{
			"consumer": map[string]any{
				defaultCommandName: map[string]any{"timeout": 225, "max-concurrent-requests": 15},
				resourceName:       map[string]any{"timeout": 175},
			},
			"provider": map[string]any{
				defaultCommandName: map[string]any{"timeout": 325, "max-concurrent-requests": 17},
				resourceName:       map[string]any{"timeout": 275},
			},
		},
	}

	cli, err := client.NewClient(
		client.SetClientExtensionConfigs(configs),
		client.WithExtension(WithConfig(WithCommandName(resourceName), WithTimeout(375))),
	)
	require.NoError(t, err)
	assert.NotNil(t, cli)
	clientSettings, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ClientScope, resourceName)]
	require.True(t, ok)
	assert.Equal(t, 375, int(clientSettings.Timeout.Milliseconds()))
	assert.Equal(t, 15, clientSettings.MaxConcurrentRequests)
	clientDefaultResource := "hystrix-client-default:::Run"
	require.True(t, ensureRuntimeCommand(extension.ClientScope,
		internalCommandName(extension.ClientScope, clientDefaultResource)))
	clientDefault, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ClientScope, clientDefaultResource)]
	require.True(t, ok)
	assert.Equal(t, 225, int(clientDefault.Timeout.Milliseconds()))
	assert.Equal(t, 15, clientDefault.MaxConcurrentRequests)

	srv, err := server.NewServer(server.SetServerExtensionConfigs(configs))
	require.NoError(t, err)
	assert.NotNil(t, srv)
	providerSettings, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ServerScope, resourceName)]
	require.True(t, ok)
	assert.Equal(t, 275, int(providerSettings.Timeout.Milliseconds()))
	providerDefaultResource := "hystrix-provider-default:::Run"
	require.True(t, ensureRuntimeCommand(extension.ServerScope,
		internalCommandName(extension.ServerScope, providerDefaultResource)))
	providerDefault, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ServerScope, providerDefaultResource)]
	require.True(t, ok)
	assert.Equal(t, 325, int(providerDefault.Timeout.Milliseconds()))
	assert.Equal(t, 17, providerDefault.MaxConcurrentRequests)
}

func isolateRuntimeConfiguration(t *testing.T) {
	t.Helper()
	hystrixConfigMu.Lock()
	commands := make(map[string]commandRuntimeState, len(hystrixRuntimeCommands))
	for name, state := range hystrixRuntimeCommands {
		commands[name] = state
	}
	defaults := make(map[extension.Scope]runtimeDefaultState, len(hystrixRuntimeDefaults))
	for scope, state := range hystrixRuntimeDefaults {
		defaults[scope] = state
	}
	hystrixConfigMu.Unlock()
	t.Cleanup(func() {
		hystrixConfigMu.Lock()
		hystrixRuntimeCommands = commands
		hystrixRuntimeDefaults = defaults
		hystrixConfigMu.Unlock()
		hystrixgo.Flush()
	})
}
