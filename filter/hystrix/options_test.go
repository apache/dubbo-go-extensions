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
	"testing"
)

import (
	"dubbo.apache.org/dubbo-go/v3/client"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
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

	var nilConfig *Config
	assert.Error(t, WithConfig().Apply(nilConfig))
	assert.Error(t, WithConfig(nil).Apply(&Config{}))
	assert.Error(t, WithConfig(WithTimeout(10)).Apply(&Config{}))
	assert.Error(t, WithConfig(WithCommandName("invalid:::Run"), WithTimeout(-1)).Apply(&Config{}))
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
	const resourceName = "hystrix-yaml-integration:::Run"
	configs := map[string]any{
		hystrixExtensionName: map[string]any{
			"consumer": map[string]any{resourceName: map[string]any{"timeout": 175}},
			"provider": map[string]any{resourceName: map[string]any{"timeout": 275}},
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

	srv, err := server.NewServer(server.SetServerExtensionConfigs(configs))
	require.NoError(t, err)
	assert.NotNil(t, srv)
	providerSettings, ok := hystrixgo.GetCircuitSettings()[internalCommandName(extension.ServerScope, resourceName)]
	require.True(t, ok)
	assert.Equal(t, 275, int(providerSettings.Timeout.Milliseconds()))
}
