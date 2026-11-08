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
	"errors"
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

	// Deprecated: use constant.HystrixConsumerFilterKey.
	HystrixConsumerFilterKey = constant.HystrixConsumerFilterKey
	// Deprecated: use constant.HystrixProviderFilterKey.
	HystrixProviderFilterKey = constant.HystrixProviderFilterKey
	// Deprecated: use the internal provider namespace managed by this package.
	DefaultProviderPrefix = internalProviderCommandPrefix
	// Deprecated: use the internal consumer namespace managed by this package.
	DefaultConsumerPrefix = internalConsumerCommandPrefix
)

func init() {
	extension.MustRegisterConfig(&Config{})
	extension.SetFilter(constant.HystrixConsumerFilterKey, newFilterConsumer)
	extension.SetFilter(constant.HystrixProviderFilterKey, newFilterProvider)
}

// FilterError wraps an error returned by Hystrix and reports whether the
// circuit was open for the request.
type FilterError struct {
	err         error
	circuitOpen bool
}

func (hfError *FilterError) Error() string {
	return hfError.err.Error()
}

// Unwrap returns the underlying Hystrix or invocation error.
func (hfError *FilterError) Unwrap() error {
	return hfError.err
}

// CircuitOpen reports whether the request failed because the circuit was
// open. It is false for timeouts, concurrency rejections, and cancellations.
func (hfError *FilterError) CircuitOpen() bool {
	return hfError.circuitOpen
}

// NewHystrixFilterError returns a FilterError instance.
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
	if !ensureRuntimeCommand(scope, cmdName) {
		return invoker.Invoke(ctx, invocation)
	}

	var (
		res          result.Result
		resMu        sync.Mutex
		fallbackUsed bool
	)
	err := hystrix.DoC(ctx, cmdName, func(ctx context.Context) error {
		invokeResult := invoker.Invoke(ctx, invocation)
		if invokeResult == nil {
			return errors.New("hystrix invoker returned a nil result")
		}
		resMu.Lock()
		if !fallbackUsed {
			res = invokeResult
		}
		resMu.Unlock()
		return invokeResult.Error()
	}, func(_ context.Context, err error) error {
		// Return fallback error
		isCircuitOpen := errors.Is(err, hystrix.ErrCircuitOpen)
		if isCircuitOpen {
			logger.Debugf("[Hystrix Filter] Circuit opened for %s", cmdName)
		} else {
			logger.Debugf("[Hystrix Filter] Hystrix fallback for %s: %v", cmdName, err)
		}
		resMu.Lock()
		fallbackUsed = true
		res = &result.RPCResult{}
		res.SetResult(nil)
		res.SetError(NewHystrixFilterError(err, isCircuitOpen))
		resMu.Unlock()
		return err
	})

	resMu.Lock()
	if res == nil {
		if err == nil {
			err = errors.New("hystrix returned no result")
		}
		res = &result.RPCResult{}
		res.SetError(NewHystrixFilterError(err, errors.Is(err, hystrix.ErrCircuitOpen)))
	}
	resMu.Unlock()
	return res
}

// warnUnconfiguredLegacyCommands reports old hystrix-go settings that the
// extension config did not declare. The caller holds hystrixConfigMu.
func warnUnconfiguredLegacyCommands(
	scope extension.Scope,
	settings map[string]*hystrix.Settings,
	configured map[string]CommandConfig,
) {
	prefix := internalConsumerCommandPrefix
	if scope == extension.ServerScope {
		prefix = internalProviderCommandPrefix
	}
	for commandName := range settings {
		if !strings.HasPrefix(commandName, prefix) {
			continue
		}
		if _, configuredByExtension := configured[commandName]; configuredByExtension {
			continue
		}
		if _, registered := hystrixRuntimeCommands[commandName]; registered {
			continue
		}
		warnLegacyCommandOnce(commandName)
	}
}

func warnLegacyCommandOnce(commandName string) {
	warning, _ := hystrixLegacyWarnings.LoadOrStore(commandName, &sync.Once{})
	warning.(*sync.Once).Do(func() {
		if _, exists := hystrix.GetCircuitSettings()[commandName]; exists {
			logger.Warnf(
				"[Hystrix Filter] existing command %q is not declared through extension config and will be bypassed; migrate it to hystrix.WithConfig or YAML",
				commandName,
			)
		}
	})
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
