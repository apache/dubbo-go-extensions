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

// Package hystrix provides a Hystrix circuit-breaker filter configured through
// the dubbo-go extension API. Command names use the Dubbo resource format
// interface:group:version:method.

package hystrix

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/filter"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"

	"github.com/afex/hystrix-go/hystrix"

	"github.com/dubbogo/gost/log/logger"
)

const (
	// Internal Hystrix command namespaces, not part of the extension config API.
	internalProviderCommandPrefix = "dubbo:provider:"
	internalConsumerCommandPrefix = "dubbo:consumer:"
)

func init() {
	extension.MustRegisterConfig(&Config{})
	extension.SetFilter(constant.HystrixConsumerFilterKey, newFilterConsumer)
	extension.SetFilter(constant.HystrixProviderFilterKey, newFilterProvider)
}

// FilterError implements error interface
type FilterError struct {
	err         error
	circuitOpen bool
}

func (hfError *FilterError) Error() string {
	return hfError.err.Error()
}

// CircuitOpen returns whether the circuit is open
func (hfError *FilterError) CircuitOpen() bool {
	return hfError.circuitOpen
}

// NewHystrixFilterError return a FilterError instance
func NewHystrixFilterError(err error, circuitOpen bool) error {
	return &FilterError{
		err:         err,
		circuitOpen: circuitOpen,
	}
}

// Filter for Hystrix
type Filter struct {
	isConsumer bool // true for consumer, false for provider
}

// Invoke is an implementation of filter, provides Hystrix pattern latency and fault tolerance
func (f *Filter) Invoke(ctx context.Context, invoker base.Invoker, invocation base.Invocation) result.Result {
	resourceName := getResourceName(invoker, invocation)
	scope := extension.ServerScope
	if f.isConsumer {
		scope = extension.ClientScope
	}
	cmdName := internalCommandName(scope, resourceName)

	var (
		res          result.Result
		resMu        sync.Mutex
		fallbackUsed bool
	)
	err := hystrix.Do(cmdName, func() error {
		invokeResult := invoker.Invoke(ctx, invocation)
		resMu.Lock()
		if !fallbackUsed {
			res = invokeResult
		}
		resMu.Unlock()
		return invokeResult.Error()
	}, func(err error) error {
		// Return fallback error
		_, isCircuitOpen := err.(hystrix.CircuitError)
		if isCircuitOpen {
			logger.Debugf("[Hystrix Filter] Circuit opened for %s", resourceName)
		} else {
			logger.Debugf("[Hystrix Filter] Hystrix fallback for %s: %v", resourceName, err)
		}
		fallbackResult := &result.RPCResult{}
		fallbackResult.SetResult(nil)
		fallbackResult.SetError(NewHystrixFilterError(err, isCircuitOpen))
		resMu.Lock()
		fallbackUsed = true
		res = fallbackResult
		resMu.Unlock()
		return err
	})

	resMu.Lock()
	defer resMu.Unlock()
	if err != nil {
		return res
	}
	return res
}

// OnResponse dummy process, returns the result directly
func (f *Filter) OnResponse(ctx context.Context, result result.Result, invoker base.Invoker, invocation base.Invocation) result.Result {
	return result
}

// newFilterConsumer returns Filter instance for consumer
func newFilterConsumer() filter.Filter {
	return &Filter{isConsumer: true}
}

// newFilterProvider returns Filter instance for provider
func newFilterProvider() filter.Filter {
	return &Filter{isConsumer: false}
}

func getResourceName(invoker base.Invoker, invocation base.Invocation) string {
	var sb strings.Builder
	// Format: interface:group:version:method
	sb.WriteString(getColonSeparatedKey(invoker.GetURL()))
	sb.WriteString(":")
	sb.WriteString(invocation.MethodName())

	return sb.String()
}

func getColonSeparatedKey(url *common.URL) string {
	return fmt.Sprintf("%s:%s:%s",
		url.Service(),
		url.GetParam(constant.GroupKey, ""),
		url.GetParam(constant.VersionKey, ""))
}
