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
	"fmt"
	"testing"
)

import (
	clusterpkg "dubbo.apache.org/dubbo-go/v3/cluster/cluster"
	"dubbo.apache.org/dubbo-go/v3/cluster/directory"
	"dubbo.apache.org/dubbo-go/v3/cluster/loadbalance"
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	protocolbase "dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

import (
	"github.com/apache/dubbo-go-extensions/cluster/metrics"
)

var _ directory.Directory = (*stubDirectory)(nil)
var _ protocolbase.Invoker = (*stubInvoker)(nil)
var _ loadbalance.LoadBalance = (*stubLoadBalance)(nil)
var _ metrics.Metrics = (*recordingMetrics)(nil)

type stubDirectory struct {
	url      *common.URL
	invokers []protocolbase.Invoker
}

func (d *stubDirectory) GetURL() *common.URL {
	return d.url
}

func (d *stubDirectory) IsAvailable() bool {
	return true
}

func (d *stubDirectory) Destroy() {}

func (d *stubDirectory) List(protocolbase.Invocation) []protocolbase.Invoker {
	return d.invokers
}

func (d *stubDirectory) Subscribe(*common.URL) error {
	return nil
}

type stubInvoker struct {
	*protocolbase.BaseInvoker
	res    result.Result
	called bool
}

func newStubInvoker(url *common.URL, res result.Result) *stubInvoker {
	return &stubInvoker{
		BaseInvoker: protocolbase.NewBaseInvoker(url),
		res:         res,
	}
}

func (i *stubInvoker) Invoke(context.Context, protocolbase.Invocation) result.Result {
	i.called = true
	return i.res
}

type stubLoadBalance struct {
	selected protocolbase.Invoker
}

func (l *stubLoadBalance) Select([]protocolbase.Invoker, protocolbase.Invocation) protocolbase.Invoker {
	return l.selected
}

type recordingMetrics struct {
	setCalls int
	lastURL  *common.URL
	method   string
	key      string
	value    any
	setErr   error
}

func (m *recordingMetrics) GetMethodMetrics(*common.URL, string, string) (any, error) {
	return nil, metrics.ErrMetricsNotFound
}

func (m *recordingMetrics) SetMethodMetrics(url *common.URL, methodName, key string, value any) error {
	m.setCalls++
	m.lastURL = url
	m.method = methodName
	m.key = key
	m.value = value
	return m.setErr
}

func (m *recordingMetrics) GetInvokerMetrics(*common.URL, string) (any, error) {
	panic("implement me")
}

func (m *recordingMetrics) SetInvokerMetrics(*common.URL, string, any) error {
	panic("implement me")
}

func (m *recordingMetrics) GetInstanceMetrics(*common.URL, string) (any, error) {
	panic("implement me")
}

func (m *recordingMetrics) SetInstanceMetrics(*common.URL, string, any) error {
	panic("implement me")
}

func TestAdaptiveServiceClusterInvoker_UnsupportedLoadBalance(t *testing.T) {
	u, err := common.NewURL("dubbo://127.0.0.1:20000/com.test.Service?loadbalance=random")
	require.NoError(t, err)
	invoker := newStubInvoker(u, &result.RPCResult{Rest: "ok"})
	ivk := newAdaptiveServiceClusterInvoker(&stubDirectory{url: u, invokers: []protocolbase.Invoker{invoker}})

	res := ivk.Invoke(context.Background(), invocation.NewRPCInvocation("GetInfo", nil, nil))

	require.Error(t, res.Error())
	assert.Contains(t, res.Error().Error(), "adaptive service not supports random load balance")
	assert.False(t, invoker.called)
}

func TestAdaptiveServiceClusterInvoker_MetricsUpdate(t *testing.T) {
	oldMetrics := metrics.LocalMetrics
	oldLoadBalance := extension.GetLoadbalance(constant.LoadBalanceKeyP2C)
	defer func() {
		metrics.LocalMetrics = oldMetrics
		extension.SetLoadbalance(constant.LoadBalanceKeyP2C, func() loadbalance.LoadBalance {
			return oldLoadBalance
		})
	}()

	u, err := common.NewURL("dubbo://127.0.0.1:20000/com.test.Service")
	require.NoError(t, err)

	run := func(t *testing.T, rpcResult *result.RPCResult, metricRecorder *recordingMetrics) result.Result {
		t.Helper()

		selected := newStubInvoker(u, rpcResult)
		extension.SetLoadbalance(constant.LoadBalanceKeyP2C, func() loadbalance.LoadBalance {
			return &stubLoadBalance{selected: selected}
		})
		metrics.LocalMetrics = metricRecorder

		ivk := newAdaptiveServiceClusterInvoker(&stubDirectory{url: u, invokers: []protocolbase.Invoker{selected}})
		invoc := invocation.NewRPCInvocation("GetInfo", nil, nil)

		res := ivk.Invoke(context.Background(), invoc)

		assert.True(t, selected.called)
		assert.Equal(t, constant.AdaptiveServiceIsEnabled,
			invoc.GetAttachmentWithDefaultValue(constant.AdaptiveServiceEnabledKey, ""))
		return res
	}

	t.Run("adaptive interruption skips metrics", func(t *testing.T) {
		recorder := &recordingMetrics{}
		res := run(t, &result.RPCResult{Err: fmt.Errorf("adaptive service interrupted: reach limitation")}, recorder)

		require.Error(t, res.Error())
		assert.Zero(t, recorder.setCalls)
	})

	t.Run("empty remaining skips metrics", func(t *testing.T) {
		recorder := &recordingMetrics{}
		res := run(t, &result.RPCResult{Rest: "ok"}, recorder)

		require.NoError(t, res.Error())
		assert.Zero(t, recorder.setCalls)
	})

	t.Run("invalid remaining skips metrics", func(t *testing.T) {
		recorder := &recordingMetrics{}
		rpcResult := &result.RPCResult{Rest: "ok"}
		rpcResult.AddAttachment(constant.AdaptiveServiceRemainingKey, "invalid")

		res := run(t, rpcResult, recorder)

		require.NoError(t, res.Error())
		assert.Zero(t, recorder.setCalls)
	})

	t.Run("string remaining writes metrics", func(t *testing.T) {
		recorder := &recordingMetrics{}
		rpcResult := &result.RPCResult{Rest: "ok"}
		rpcResult.AddAttachment(constant.AdaptiveServiceRemainingKey, "42")

		res := run(t, rpcResult, recorder)

		require.NoError(t, res.Error())
		require.Equal(t, 1, recorder.setCalls)
		assert.Equal(t, u, recorder.lastURL)
		assert.Equal(t, "GetInfo", recorder.method)
		assert.Equal(t, metrics.HillClimbing, recorder.key)
		assert.Equal(t, uint64(42), recorder.value)
	})

	t.Run("string slice remaining writes metrics", func(t *testing.T) {
		recorder := &recordingMetrics{}
		rpcResult := &result.RPCResult{Rest: "ok"}
		rpcResult.AddAttachment(constant.AdaptiveServiceRemainingKey, []string{"7"})

		res := run(t, rpcResult, recorder)

		require.NoError(t, res.Error())
		require.Equal(t, 1, recorder.setCalls)
		assert.Equal(t, uint64(7), recorder.value)
	})

	t.Run("metrics write failure returns error result", func(t *testing.T) {
		writeErr := errors.New("write failed")
		recorder := &recordingMetrics{setErr: writeErr}
		rpcResult := &result.RPCResult{Rest: "ok"}
		rpcResult.AddAttachment(constant.AdaptiveServiceRemainingKey, "1")

		res := run(t, rpcResult, recorder)

		require.Error(t, res.Error())
		assert.ErrorIs(t, res.Error(), writeErr)
	})
}

func TestAdaptiveServiceClusterInvoker_Availability(t *testing.T) {
	for _, tc := range []struct {
		name         string
		available    []bool
		destroyFirst bool
		wantProvider int
	}{
		{"reselect healthy provider", []bool{false, true}, false, 1},
		{"skip destroyed provider", []bool{false, true}, true, 1},
		{"single unavailable provider", []bool{false}, false, -1},
		{"all providers unavailable", []bool{false, false}, false, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldMetrics := metrics.LocalMetrics
			recorder := &recordingMetrics{}
			metrics.LocalMetrics = recorder
			t.Cleanup(func() { metrics.LocalMetrics = oldMetrics })

			providers := make([]*stubInvoker, len(tc.available))
			invokers := make([]protocolbase.Invoker, len(tc.available))
			for i, available := range tc.available {
				u, err := common.NewURL(fmt.Sprintf("dubbo://127.0.0.1:%d/com.test.Availability", 20000+i))
				require.NoError(t, err)
				res := &result.RPCResult{Rest: i}
				res.AddAttachment(constant.AdaptiveServiceRemainingKey, "10")
				providers[i] = newStubInvoker(u, res)
				providers[i].SetAvailable(available)
				invokers[i] = providers[i]
			}
			dir := &stubDirectory{url: providers[0].GetURL(), invokers: invokers}
			if tc.destroyFirst {
				providers[0].Destroy()
			}
			t.Cleanup(func() {
				for _, provider := range providers {
					protocolbase.RemoveInvokerUnhealthyStatus(provider)
				}
			})
			ivk := newAdaptiveServiceClusterInvoker(dir)

			// Real P2C initially picks the first provider when neither has metrics.
			// Repeat to catch repeatedly selecting an unavailable, unmeasured provider.
			for range 10 {
				for _, provider := range providers {
					provider.called = false
				}
				res := ivk.Invoke(context.Background(), invocation.NewRPCInvocation("GetInfo", nil, nil))
				if tc.wantProvider < 0 {
					require.Error(t, res.Error())
					assert.Zero(t, recorder.setCalls)
				} else {
					require.NoError(t, res.Error())
					require.Equal(t, tc.wantProvider, res.Result())
					assert.Same(t, providers[tc.wantProvider].GetURL(), recorder.lastURL)
				}
				for i, provider := range providers {
					assert.Equal(t, i == tc.wantProvider, provider.called, "provider %d", i)
				}
			}
		})
	}
}

type dummyCluster struct{}

func TestAdaptiveServiceClusterInvoker_GenericMethodMetrics(t *testing.T) {
	u0, err := common.NewURL("dubbo://127.0.0.1:21000/" + t.Name())
	require.NoError(t, err)
	u1, err := common.NewURL("dubbo://127.0.0.1:21001/" + t.Name())
	require.NoError(t, err)
	providers := []protocolbase.Invoker{
		newStubInvoker(u0, nil),
		newStubInvoker(u1, nil),
	}
	genericInvocation := func(method string) *invocation.RPCInvocation {
		return invocation.NewRPCInvocation(constant.Generic, []any{method, []string{}, []any{}}, nil)
	}
	cases := []struct {
		method    string
		remaining [2]uint64
		preferred int
	}{
		{"Echo", [2]uint64{10, 30}, 1},
		{"Lookup", [2]uint64{40, 5}, 0},
	}
	for _, tc := range cases {
		for i, provider := range providers {
			res := &result.RPCResult{Rest: "ok"}
			res.AddAttachment(constant.AdaptiveServiceRemainingKey, fmt.Sprint(tc.remaining[i]))
			provider.(*stubInvoker).res = res
			caller := newAdaptiveServiceClusterInvoker(&stubDirectory{
				url: provider.GetURL(), invokers: []protocolbase.Invoker{provider},
			})
			inv := genericInvocation(tc.method)
			require.Equal(t, constant.Generic, inv.MethodName())
			require.Equal(t, tc.method, inv.ActualMethodName())
			require.NoError(t, caller.Invoke(context.Background(), inv).Error())
		}
	}
	for _, tc := range cases {
		for i, provider := range providers {
			value, err := metrics.LocalMetrics.GetMethodMetrics(provider.GetURL(), tc.method, metrics.HillClimbing)
			require.NoError(t, err)
			require.Equal(t, tc.remaining[i], value)
			_, err = metrics.LocalMetrics.GetMethodMetrics(provider.GetURL(), constant.Generic, metrics.HillClimbing)
			require.ErrorIs(t, err, metrics.ErrMetricsNotFound)
		}
		lb := extension.GetLoadbalance(constant.LoadBalanceKeyP2C)
		require.Same(t, providers[tc.preferred], lb.Select(providers, genericInvocation(tc.method)))
	}
}

func (dummyCluster) Join(directory.Directory) protocolbase.Invoker {
	return protocolbase.NewBaseInvoker(nil)
}

func TestAdaptiveServiceCluster_RegistrationOverwrite(t *testing.T) {
	extension.SetCluster(constant.ClusterKeyAdaptiveService, func() clusterpkg.Cluster {
		return dummyCluster{}
	})

	require.NotPanics(t, registerAdaptiveServiceCluster)
	registered, err := extension.GetCluster(constant.ClusterKeyAdaptiveService)
	require.NoError(t, err)
	assert.IsType(t, &adaptiveServiceCluster{}, registered)
}
