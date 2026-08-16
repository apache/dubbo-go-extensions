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

// Package adaptivesvc providers AdaptiveService filter.
package adaptivesvc

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

import (
	"github.com/dubbogo/gost/log/logger"

	"github.com/pkg/errors"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/filter"
	"dubbo.apache.org/dubbo-go/v3/global"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"
	"github.com/apache/dubbo-go-extensions/filter/adaptivesvc/limiter"
	internaladaptive "github.com/apache/dubbo-go-extensions/internal/adaptivesvc"
)

var (
	adaptiveServiceProviderFilterOnce sync.Once
	instance                          filter.Filter

	ErrAdaptiveSvcInterrupted = fmt.Errorf(internaladaptive.AdaptiveServiceInterruptedMessage)
	ErrUpdaterNotFound        = fmt.Errorf("updater not found")
	ErrUnexpectedUpdaterType  = fmt.Errorf("unexpected updater type")
)

func init() {
	registerAdaptiveServiceProviderFilter()
}

func registerAdaptiveServiceProviderFilter() {
	extension.SetFilter(internaladaptive.ProviderFilterKey, newAdaptiveServiceProviderFilter)
}

// adaptiveServiceProviderFilter is for adaptive service on the provider side.
type adaptiveServiceProviderFilter struct{}

func newAdaptiveServiceProviderFilter() filter.Filter {
	if instance == nil {
		adaptiveServiceProviderFilterOnce.Do(func() {
			instance = &adaptiveServiceProviderFilter{}
		})
	}
	return instance
}

func (f *adaptiveServiceProviderFilter) Invoke(ctx context.Context, invoker base.Invoker,
	invocation base.Invocation) result.Result {
	if invocation.GetAttachmentWithDefaultValue(internaladaptive.EnabledKey, "") !=
		internaladaptive.EnabledValue {
		// the adaptive service is enabled on the invocation
		return invoker.Invoke(ctx, invocation)
	}
	configureLimiterVerbose(invoker)

	l, err := limiterMapperSingleton.getMethodLimiter(invoker.GetURL(), invocation.MethodName())
	if err != nil {
		if errors.Is(err, ErrLimiterNotFoundOnMapper) {
			// limiter is not found on the mapper, just create
			// a new limiter
			if l, err = limiterMapperSingleton.newAndSetMethodLimiter(invoker.GetURL(),
				invocation.MethodName(), limiter.HillClimbingLimiter); err != nil {
				return &result.RPCResult{Err: wrapErrAdaptiveSvcInterrupted(err)}
			}
		} else {
			// unexpected errors
			return &result.RPCResult{Err: wrapErrAdaptiveSvcInterrupted(err)}
		}
	}

	updater, err := l.Acquire()
	if err != nil {
		return &result.RPCResult{Err: wrapErrAdaptiveSvcInterrupted(err)}
	}

	invocation.SetAttribute(internaladaptive.UpdaterKey, updater)
	return invoker.Invoke(ctx, invocation)
}

func configureLimiterVerbose(invoker base.Invoker) {
	providerConfigRaw, ok := invoker.GetURL().GetAttribute(constant.ProviderConfigKey)
	if !ok {
		return
	}
	providerConfig, ok := providerConfigRaw.(*global.ProviderConfig)
	if !ok || !providerConfig.AdaptiveServiceVerbose {
		return
	}
	limiter.Verbose = true
}

func (f *adaptiveServiceProviderFilter) OnResponse(_ context.Context, res result.Result, invoker base.Invoker,
	invocation base.Invocation) result.Result {
	var asEnabled string
	asEnabledIface := res.Attachment(internaladaptive.EnabledKey, nil)
	if asEnabledIface != nil {
		if str, strOK := asEnabledIface.(string); strOK {
			asEnabled = str
		} else if strArr, strArrOK := asEnabledIface.([]string); strArrOK && len(strArr) > 0 {
			asEnabled = strArr[0]
		}
	}
	if asEnabled != internaladaptive.EnabledValue {
		// the adaptive service is enabled on the invocation
		return res
	}

	if isErrAdaptiveSvcInterrupted(res.Error()) {
		// If the Invoke method of the adaptiveServiceProviderFilter returns an error,
		// the OnResponse of the adaptiveServiceProviderFilter should not be performed.
		return res
	}

	// get updater from the attributes
	updaterIface, _ := invocation.GetAttribute(internaladaptive.UpdaterKey)
	if updaterIface == nil {
		logger.Errorf("[Filter][AdaptiveSvc] the updater is not found on the attributes, attrs=%#v",
			invocation.Attributes())
		return &result.RPCResult{Err: ErrUpdaterNotFound}
	}
	updater, ok := updaterIface.(limiter.Updater)
	if !ok {
		logger.Errorf("[Filter][AdaptiveSvc] the type of the updater is not unexpected, we got %#v", updaterIface)
		return &result.RPCResult{Err: ErrUnexpectedUpdaterType}
	}

	err := updater.DoUpdate()
	if err != nil {
		logger.Errorf("[Filter][AdaptiveSvc] the DoUpdate method failed, err=%v", err)
		return &result.RPCResult{Err: err}
	}

	// get limiter for the mapper
	l, err := limiterMapperSingleton.getMethodLimiter(invoker.GetURL(), invocation.MethodName())
	if err != nil {
		logger.Errorf("[Filter][AdaptiveSvc] the method limiter for %q is not found.", invocation.MethodName())
		return &result.RPCResult{Err: err}
	}

	// set attachments to inform consumer of provider status
	res.AddAttachment(internaladaptive.RemainingKey, fmt.Sprintf("%d", l.Remaining()))
	res.AddAttachment(internaladaptive.InflightKey, fmt.Sprintf("%d", l.Inflight()))
	logger.Debugf("[Filter][AdaptiveSvc] the attachments are set, %s=%d %s=%d.",
		internaladaptive.RemainingKey, l.Remaining(),
		internaladaptive.InflightKey, l.Inflight())

	return res
}

func wrapErrAdaptiveSvcInterrupted(customizedErr any) error {
	return fmt.Errorf("%w: %v", ErrAdaptiveSvcInterrupted, customizedErr)
}

func isErrAdaptiveSvcInterrupted(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), ErrAdaptiveSvcInterrupted.Error())
}
