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
	"errors"
	"testing"
	"time"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"

	"github.com/afex/hystrix-go/hystrix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHystrixFilterError(t *testing.T) {
	get := NewHystrixFilterError(errors.New("test"), true)
	assert.True(t, get.(*FilterError).CircuitOpen())
	assert.Equal(t, "test", get.Error())
}

func TestGetHystrixFilter(t *testing.T) {
	filterGot := newFilterConsumer()
	assert.NotNil(t, filterGot)

	filterGot = newFilterProvider()
	assert.NotNil(t, filterGot)
}

func TestHystrixFilter_Invoke(t *testing.T) {
	resourceName := "com.ikurento.user.UserProvider:::TestMethod"
	config := (&Config{}).New().(*Config)
	config.Commands[resourceName] = CommandConfig{
		Timeout:                1000,
		MaxConcurrentRequests:  10,
		RequestVolumeThreshold: 5,
		SleepWindow:            1000,
		ErrorPercentThreshold:  50,
	}
	require.NoError(t, config.Init(extension.ClientScope))
	t.Cleanup(func() {
		assert.NoError(t, config.Rollback(extension.ClientScope))
		hystrix.Flush()
	})

	url, err := common.NewURL("dubbo://127.0.0.1:20000/com.ikurento.user.UserProvider")
	require.NoError(t, err)
	mockInvoker := base.NewBaseInvoker(url)

	filter := &Filter{isConsumer: true}
	mockInvocation := invocation.NewRPCInvocation("TestMethod", []any{"OK"}, make(map[string]any))

	ctx := context.Background()
	result := filter.Invoke(ctx, mockInvoker, mockInvocation)
	assert.NotNil(t, result)
	assert.NoError(t, result.Error())
}

func TestGetResourceName(t *testing.T) {
	url, err := common.NewURL("dubbo://127.0.0.1:20000/com.ikurento.user.UserProvider")
	require.NoError(t, err)
	invoker := base.NewBaseInvoker(url)
	invocation := invocation.NewRPCInvocation("TestMethod", []any{}, make(map[string]any))

	assert.Equal(t, "com.ikurento.user.UserProvider:::TestMethod", getResourceName(invoker, invocation))
	assert.Equal(t, "dubbo:consumer:com.ikurento.user.UserProvider:::TestMethod",
		internalCommandName(extension.ClientScope, getResourceName(invoker, invocation)))
	assert.Equal(t, "dubbo:provider:com.ikurento.user.UserProvider:::TestMethod",
		internalCommandName(extension.ServerScope, getResourceName(invoker, invocation)))
}

func TestHystrixFilterTimeoutAndCircuitFallback(t *testing.T) {
	const resourceName = "hystrix-timeout-test:::Run"
	config := (&Config{}).New().(*Config)
	config.Commands[resourceName] = CommandConfig{
		Timeout:                100,
		MaxConcurrentRequests:  10,
		RequestVolumeThreshold: 1,
		SleepWindow:            60000,
		ErrorPercentThreshold:  1,
	}
	require.NoError(t, config.Init(extension.ClientScope))
	t.Cleanup(func() {
		assert.NoError(t, config.Rollback(extension.ClientScope))
		hystrix.Flush()
	})

	url, err := common.NewURL("dubbo://127.0.0.1:20000/hystrix-timeout-test")
	require.NoError(t, err)
	invoker := &delayedInvoker{Invoker: base.NewBaseInvoker(url), delay: 300 * time.Millisecond}
	hystrixFilter := &Filter{isConsumer: true}
	invocation := invocation.NewRPCInvocation("Run", []any{}, make(map[string]any))

	first := hystrixFilter.Invoke(context.Background(), invoker, invocation)
	require.Error(t, first.Error())
	var firstError *FilterError
	require.True(t, errors.As(first.Error(), &firstError))
	assert.False(t, firstError.CircuitOpen())
	assert.ErrorIs(t, first.Error(), hystrix.ErrTimeout)

	second := hystrixFilter.Invoke(context.Background(), invoker, invocation)
	require.Error(t, second.Error())
	var secondError *FilterError
	require.True(t, errors.As(second.Error(), &secondError))
	assert.True(t, secondError.CircuitOpen())
	assert.ErrorIs(t, second.Error(), hystrix.ErrCircuitOpen)
}

func TestHystrixFilterContextCancellation(t *testing.T) {
	const resourceName = "hystrix-cancel-test:::Run"
	config := (&Config{}).New().(*Config)
	config.Commands[resourceName] = CommandConfig{
		Timeout:                1000,
		MaxConcurrentRequests:  10,
		RequestVolumeThreshold: 5,
		SleepWindow:            1000,
		ErrorPercentThreshold:  50,
	}
	require.NoError(t, config.Init(extension.ClientScope))
	t.Cleanup(func() {
		assert.NoError(t, config.Rollback(extension.ClientScope))
		hystrix.Flush()
	})

	url, err := common.NewURL("dubbo://127.0.0.1:20000/hystrix-cancel-test")
	require.NoError(t, err)
	invoker := &contextAwareInvoker{Invoker: base.NewBaseInvoker(url), started: make(chan struct{})}
	hystrixFilter := &Filter{isConsumer: true}
	invocation := invocation.NewRPCInvocation("Run", []any{}, make(map[string]any))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan result.Result, 1)
	go func() {
		resultCh <- hystrixFilter.Invoke(ctx, invoker, invocation)
	}()
	<-invoker.started
	cancel()

	select {
	case invokeResult := <-resultCh:
		require.Error(t, invokeResult.Error())
		assert.ErrorIs(t, invokeResult.Error(), context.Canceled)
		var filterError *FilterError
		require.True(t, errors.As(invokeResult.Error(), &filterError))
		assert.False(t, filterError.CircuitOpen())
	case <-time.After(time.Second):
		t.Fatal("Invoke did not return after context cancellation")
	}
}

func TestHystrixFilterPassesThroughUnconfiguredMethod(t *testing.T) {
	const resourceName = "hystrix-unconfigured-test:::Run"
	url, err := common.NewURL("dubbo://127.0.0.1:20000/hystrix-unconfigured-test")
	require.NoError(t, err)
	invoker := base.NewBaseInvoker(url)
	hystrixFilter := &Filter{isConsumer: true}
	invocation := invocation.NewRPCInvocation("Run", []any{}, make(map[string]any))
	commandName := internalCommandName(extension.ClientScope, resourceName)

	result := hystrixFilter.Invoke(context.Background(), invoker, invocation)
	require.NotNil(t, result)
	assert.NoError(t, result.Error())
	_, configured := hystrix.GetCircuitSettings()[commandName]
	assert.False(t, configured)
}

type delayedInvoker struct {
	base.Invoker
	delay time.Duration
}

func (invoker *delayedInvoker) Invoke(context.Context, base.Invocation) result.Result {
	time.Sleep(invoker.delay)
	return &result.RPCResult{}
}

type contextAwareInvoker struct {
	base.Invoker
	started chan struct{}
}

func (invoker *contextAwareInvoker) Invoke(ctx context.Context, _ base.Invocation) result.Result {
	close(invoker.started)
	<-ctx.Done()
	invokeResult := &result.RPCResult{}
	invokeResult.SetError(ctx.Err())
	return invokeResult
}

func TestHystrixFilter_OnResponse(t *testing.T) {
	filter := &Filter{isConsumer: true}
	ctx := context.Background()
	url, err := common.NewURL("dubbo://127.0.0.1:20000/com.ikurento.user.UserProvider")
	require.NoError(t, err)
	mockInvoker := base.NewBaseInvoker(url)
	mockInvocation := invocation.NewRPCInvocation("TestMethod", []any{}, make(map[string]any))

	result := filter.OnResponse(ctx, nil, mockInvoker, mockInvocation)
	assert.Nil(t, result)
}
