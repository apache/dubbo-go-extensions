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

package metrics

import (
	"testing"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalMetrics_MethodMetricsAreScopedByProviderAndMethod(t *testing.T) {
	metrics := newLocalMetrics()

	firstProvider, err := common.NewURL("dubbo://192.168.1.10:20880/com.example.Greeter")
	require.NoError(t, err)
	secondProvider, err := common.NewURL("dubbo://192.168.1.11:20880/com.example.Greeter")
	require.NoError(t, err)

	require.NoError(t, metrics.SetMethodMetrics(firstProvider, "SayHello", HillClimbing, uint64(42)))
	require.NoError(t, metrics.SetMethodMetrics(firstProvider, "SayGoodbye", HillClimbing, uint64(7)))
	require.NoError(t, metrics.SetMethodMetrics(secondProvider, "SayHello", HillClimbing, uint64(13)))

	value, err := metrics.GetMethodMetrics(firstProvider, "SayHello", HillClimbing)
	require.NoError(t, err)
	assert.Equal(t, uint64(42), value)

	value, err = metrics.GetMethodMetrics(firstProvider, "SayGoodbye", HillClimbing)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), value)

	value, err = metrics.GetMethodMetrics(secondProvider, "SayHello", HillClimbing)
	require.NoError(t, err)
	assert.Equal(t, uint64(13), value)
}

func TestLocalMetrics_GetMethodMetricsReturnsNotFoundForUnknownMetric(t *testing.T) {
	metrics := newLocalMetrics()
	provider, err := common.NewURL("dubbo://192.168.1.10:20880/com.example.Greeter")
	require.NoError(t, err)

	value, err := metrics.GetMethodMetrics(provider, "SayHello", HillClimbing)
	assert.Nil(t, value)
	assert.ErrorIs(t, err, ErrMetricsNotFound)
}

func TestLocalMetrics_KeyComponents(t *testing.T) {
	provider, err := common.NewURL("dubbo://192.168.1.10:20880/com.example.Greeter")
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.10:20880", getInstanceKey(provider))
	assert.Equal(t, "/com.example.Greeter", getInvokerKey(provider))
}
