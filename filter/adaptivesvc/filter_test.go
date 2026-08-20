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

package adaptivesvc

import (
	"context"
	"errors"
	"testing"
)

import (
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/filter"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/protocol/mock"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"
	"github.com/apache/dubbo-go-extensions/filter/adaptivesvc/limiter"
)

type mockUpdater struct {
	called bool
	err    error
}

func (m *mockUpdater) DoUpdate() error {
	m.called = true
	return m.err
}

func (m *mockUpdater) Report(_ uint64) {}

type alwaysLimitLimiter struct{}

func (alwaysLimitLimiter) Inflight() uint64 {
	return 0
}

func (alwaysLimitLimiter) Remaining() uint64 {
	return 0
}

func (alwaysLimitLimiter) Acquire() (limiter.Updater, error) {
	return nil, limiter.ErrReachLimitation
}

func TestAdaptiveServiceProviderFilter_Invoke(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u, _ := common.NewURL("dubbo://127.0.0.1:20000/com.test.Service")
	methodName := "GetInfo"
	filter := newAdaptiveServiceProviderFilter()

	t.Run("AdaptiveDisabled", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		invoker := mock.NewMockInvoker(ctrl)
		invoker.EXPECT().Invoke(gomock.Any(), gomock.Any()).Return(&result.RPCResult{Rest: "ok"})

		res := filter.Invoke(context.Background(), invoker, invoc)
		assert.NoError(t, res.Error())
	})

	t.Run("AdaptiveEnabled_AcquireSuccess", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, map[string]any{
			constant.AdaptiveServiceEnabledKey: constant.AdaptiveServiceIsEnabled,
		})
		invoker := mock.NewMockInvoker(ctrl)
		invoker.EXPECT().GetURL().Return(u).AnyTimes()
		invoker.EXPECT().Invoke(gomock.Any(), gomock.Any()).Return(&result.RPCResult{Rest: "ok"})

		res := filter.Invoke(context.Background(), invoker, invoc)
		require.NoError(t, res.Error())

		updater, _ := invoc.GetAttribute(constant.AdaptiveServiceUpdaterKey)
		assert.NotNil(t, updater)
	})

	t.Run("AdaptiveEnabled_LimiterReached", func(t *testing.T) {
		oldMapper := limiterMapperSingleton
		limiterMapperSingleton = newLimiterMapper()
		defer func() {
			limiterMapperSingleton = oldMapper
		}()

		key := u.Path + methodName
		limiterMapperSingleton.mapper[key] = alwaysLimitLimiter{}

		invoc := invocation.NewRPCInvocation(methodName, nil, map[string]any{
			constant.AdaptiveServiceEnabledKey: constant.AdaptiveServiceIsEnabled,
		})
		invoker := mock.NewMockInvoker(ctrl)
		invoker.EXPECT().GetURL().Return(u).AnyTimes()

		res := filter.Invoke(context.Background(), invoker, invoc)
		require.Error(t, res.Error())
		assert.EqualError(t, res.Error(), "adaptive service interrupted: reach limitation")
	})
}

func TestAdaptiveServiceProviderFilter_OnResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u, _ := common.NewURL("dubbo://127.0.0.1:20000/com.test.Service")
	methodName := "GetInfo"
	filter := newAdaptiveServiceProviderFilter()

	t.Run("DisabledInResponse", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		res := &result.RPCResult{Rest: "ok"}
		invoker := mock.NewMockInvoker(ctrl)

		ret := filter.OnResponse(context.Background(), res, invoker, invoc)
		assert.Equal(t, res, ret)
	})

	t.Run("InterruptedErrorShouldSkip", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		res := &result.RPCResult{Err: wrapErrAdaptiveSvcInterrupted("limit exceeded")}
		invoker := mock.NewMockInvoker(ctrl)

		ret := filter.OnResponse(context.Background(), res, invoker, invoc)
		assert.True(t, isErrAdaptiveSvcInterrupted(ret.Error()))
	})

	t.Run("MissingUpdater", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		res := &result.RPCResult{Rest: "ok"}
		res.AddAttachment(constant.AdaptiveServiceEnabledKey, constant.AdaptiveServiceIsEnabled)
		invoker := mock.NewMockInvoker(ctrl)

		ret := filter.OnResponse(context.Background(), res, invoker, invoc)
		assert.ErrorIs(t, ret.Error(), ErrUpdaterNotFound)
	})

	t.Run("UnexpectedUpdaterType", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		invoc.SetAttribute(constant.AdaptiveServiceUpdaterKey, "bad updater")
		res := &result.RPCResult{Rest: "ok"}
		res.AddAttachment(constant.AdaptiveServiceEnabledKey, []string{constant.AdaptiveServiceIsEnabled})
		invoker := mock.NewMockInvoker(ctrl)

		ret := filter.OnResponse(context.Background(), res, invoker, invoc)
		assert.ErrorIs(t, ret.Error(), ErrUnexpectedUpdaterType)
	})

	t.Run("UpdaterError", func(t *testing.T) {
		updateErr := errors.New("update failed")
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		invoc.SetAttribute(constant.AdaptiveServiceUpdaterKey, &mockUpdater{err: updateErr})
		res := &result.RPCResult{Rest: "ok"}
		res.AddAttachment(constant.AdaptiveServiceEnabledKey, constant.AdaptiveServiceIsEnabled)
		invoker := mock.NewMockInvoker(ctrl)

		ret := filter.OnResponse(context.Background(), res, invoker, invoc)
		assert.ErrorIs(t, ret.Error(), updateErr)
	})

	t.Run("SuccessWithAttachments", func(t *testing.T) {
		invoc := invocation.NewRPCInvocation(methodName, nil, nil)
		updater := &mockUpdater{}
		invoc.SetAttribute(constant.AdaptiveServiceUpdaterKey, updater)

		res := &result.RPCResult{Rest: "ok"}
		res.AddAttachment(constant.AdaptiveServiceEnabledKey, constant.AdaptiveServiceIsEnabled)

		invoker := mock.NewMockInvoker(ctrl)
		invoker.EXPECT().GetURL().Return(u).AnyTimes()

		_, _ = limiterMapperSingleton.newAndSetMethodLimiter(u, methodName, limiter.HillClimbingLimiter)

		ret := filter.OnResponse(context.Background(), res, invoker, invoc)

		require.NoError(t, ret.Error())
		assert.NotEmpty(t, ret.Attachment(constant.AdaptiveServiceRemainingKey, ""))
		assert.NotEmpty(t, ret.Attachment(constant.AdaptiveServiceInflightKey, ""))
		assert.True(t, updater.called)
	})
}

type dummyFilter struct{}

func (dummyFilter) Invoke(context.Context, base.Invoker, base.Invocation) result.Result {
	return &result.RPCResult{}
}

func (dummyFilter) OnResponse(context.Context, result.Result, base.Invoker, base.Invocation) result.Result {
	return &result.RPCResult{}
}

func TestAdaptiveServiceProviderFilter_RegistrationOverwrite(t *testing.T) {
	extension.SetFilter(constant.AdaptiveServiceProviderFilterKey, func() filter.Filter {
		return dummyFilter{}
	})

	require.NotPanics(t, registerAdaptiveServiceProviderFilter)
	registered, ok := extension.GetFilter(constant.AdaptiveServiceProviderFilterKey)
	require.True(t, ok)
	assert.IsType(t, &adaptiveServiceProviderFilter{}, registered)
}
