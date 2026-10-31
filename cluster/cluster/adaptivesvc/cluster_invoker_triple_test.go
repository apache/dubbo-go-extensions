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
	"net"
	"net/http"
	"testing"
	"time"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	protocolbase "dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/protocol/triple"
	tri "dubbo.apache.org/dubbo-go/v3/protocol/triple/triple_protocol"

	"github.com/stretchr/testify/require"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

import (
	"github.com/apache/dubbo-go-extensions/cluster/metrics"
)

// TestAdaptiveServiceClusterInvoker_TripleResponseMetrics exercises real Triple
// transport. The handler supplies capacity directly to isolate the response
// metadata path from the provider's limiter algorithm.
func TestAdaptiveServiceClusterInvoker_TripleResponseMetrics(t *testing.T) {
	for _, tc := range []struct {
		name           string
		callerMetadata bool
	}{
		{"automatic trailer capture", false},
		{"preserve caller metadata", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := tri.NewUnaryHandler(
				"/test.AdaptiveService/Echo",
				func() any { return new(wrapperspb.StringValue) },
				func(_ context.Context, _ *tri.Request) (*tri.Response, error) {
					response := tri.NewResponse(wrapperspb.String("ok"))
					response.Header().Set("x-test-header", "header-value")
					response.Trailer().Set("x-test-trailer", "trailer-value")
					response.Trailer().Set(constant.AdaptiveServiceRemainingKey, "42")
					return response, nil
				},
			)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			server := &http.Server{Handler: h2c.NewHandler(handler, &http2.Server{})}
			t.Cleanup(func() { _ = server.Close() })
			go func() { _ = server.Serve(listener) }()

			url, err := common.NewURL(
				"tri://"+listener.Addr().String()+"/test.AdaptiveService",
				common.WithMethods([]string{"Echo"}),
				common.WithProtocol("tri"),
				common.WithParamsValue(constant.IDLMode, constant.NONIDL),
			)
			require.NoError(t, err)
			provider := triple.GetProtocol().Refer(url)
			require.NotNil(t, provider)
			t.Cleanup(provider.Destroy)
			cluster := newAdaptiveServiceClusterInvoker(&stubDirectory{
				url:      url,
				invokers: []protocolbase.Invoker{provider},
			})

			var header, trailer http.Header
			response := new(wrapperspb.StringValue)
			inv := invocation.NewRPCInvocationWithOptions(
				invocation.WithMethodName("Echo"),
				invocation.WithParameterRawValues([]any{wrapperspb.String("hello"), response}),
			)
			inv.SetAttribute(constant.CallTypeKey, constant.CallUnary)
			if tc.callerMetadata {
				inv.SetAttribute(constant.ResponseHeaderKey, &header)
				inv.SetAttribute(constant.ResponseTrailerKey, &trailer)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res := cluster.Invoke(ctx, inv)
			require.NoError(t, res.Error())
			require.Equal(t, "ok", response.Value)
			if tc.callerMetadata {
				require.Equal(t, "header-value", header.Get("x-test-header"))
				require.Equal(t, "trailer-value", trailer.Get("x-test-trailer"))
				require.Equal(t, "42", trailer.Get(constant.AdaptiveServiceRemainingKey))
			}

			remaining, err := metrics.LocalMetrics.GetMethodMetrics(url, "Echo", metrics.HillClimbing)
			t.Logf("response=%q; result attachments=%v; metric=%v; metrics error=%v",
				response.Value, res.Attachments(), remaining, err)
			require.NoError(t, err, "Triple response capacity must reach adaptive metrics")
			require.Equal(t, uint64(42), remaining)

			// Simulate a newer sample from another call, then reuse this invocation.
			// Invalid outgoing metadata fails before Triple writes response targets;
			// the old response must not overwrite the newer capacity sample.
			require.NoError(t, metrics.LocalMetrics.SetMethodMetrics(url, "Echo", metrics.HillClimbing, uint64(7)))
			inv.SetAttachment("invalid-test-header", 42)
			failed := cluster.Invoke(ctx, inv)
			require.Error(t, failed.Error())
			remaining, err = metrics.LocalMetrics.GetMethodMetrics(url, "Echo", metrics.HillClimbing)
			require.NoError(t, err)
			require.Equal(t, uint64(7), remaining, "failed call must not reuse previous response metadata")
		})
	}
}
