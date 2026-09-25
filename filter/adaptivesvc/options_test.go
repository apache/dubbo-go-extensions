/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to you under the Apache License, Version 2.0
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

package adaptivesvc

import (
	"testing"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigRegistered(t *testing.T) {
	registered, ok := extension.LookupConfig(adaptiveServiceExtensionName)
	require.True(t, ok)
	assert.IsType(t, &Config{}, registered)
}

func TestConfigNewReturnsIndependentConfig(t *testing.T) {
	prototype := &Config{}
	first := prototype.New()
	second := prototype.New()

	assert.IsType(t, &Config{}, first)
	assert.NotSame(t, first, second)
	assert.Equal(t, adaptiveServiceExtensionName, first.Prefix())
}

func TestConfigInitOnlySupportsServerScope(t *testing.T) {
	config := &Config{}

	assert.NoError(t, config.Init(extension.ServerScope))
	assert.Error(t, config.Init(extension.ClientScope))
	assert.Error(t, config.Init(extension.InstanceScope))
}

func TestConfigFilterNames(t *testing.T) {
	config := &Config{}

	assert.Equal(t, []string{constant.AdaptiveServiceProviderFilterKey},
		config.FilterNames(extension.ServerScope))
	assert.Nil(t, config.FilterNames(extension.ClientScope))
	assert.Nil(t, config.FilterNames(extension.InstanceScope))
}

func TestWithAdaptiveService(t *testing.T) {
	filterNames, err := extension.Initialize(nil,
		[]extension.Option{WithAdaptiveService()}, extension.ServerScope)

	require.NoError(t, err)
	assert.Equal(t, []string{constant.AdaptiveServiceProviderFilterKey}, filterNames)
}

func TestWithAdaptiveServiceRejectsUnsupportedScope(t *testing.T) {
	_, err := extension.Initialize(nil,
		[]extension.Option{WithAdaptiveService()}, extension.ClientScope)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supports server scope")
}

func TestWithAdaptiveServiceIntegratesWithServerOption(t *testing.T) {
	srv, err := server.NewServer(
		server.WithExtension(WithAdaptiveService()),
	)

	require.NoError(t, err)
	assert.NotNil(t, srv)
}
