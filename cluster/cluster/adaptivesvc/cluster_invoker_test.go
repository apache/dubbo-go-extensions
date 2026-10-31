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
	defer func() {
		metrics.LocalMetrics = oldMetrics
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

type dummyCluster struct{}

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
