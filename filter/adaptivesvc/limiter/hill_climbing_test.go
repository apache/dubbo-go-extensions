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

package limiter

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test for HillClimbing limiter's Acquire method
func TestHillClimbing_Acquire(t *testing.T) {
	limiter := NewHillClimbing().(*HillClimbing)

	// Simulating that there is remaining capacity
	limiter.limitation.Store(100)
	limiter.inflight.Store(50)

	updater, err := limiter.Acquire()
	assert.NotNil(t, updater)
	require.NoError(t, err)

	// Simulating no remaining capacity
	limiter.limitation.Store(50)
	limiter.inflight.Store(50)

	updater, err = limiter.Acquire()
	assert.Nil(t, updater)
	assert.ErrorIs(t, err, ErrReachLimitation)
}

// Test the HillClimbingUpdater's DoUpdate method
func TestHillClimbingUpdater_DoUpdate(t *testing.T) {
	limiter := NewHillClimbing().(*HillClimbing)
	updater, err := limiter.Acquire()
	require.NoError(t, err)

	// Simulate the limiter update with arbitrary values for RTT and inflight
	// Normally, this would adjust the limiter's limitation based on RTT and inflight metrics
	err = updater.DoUpdate()
	assert.NoError(t, err)
}

// Test adjustLimitation method with different options
func TestHillClimbingUpdater_AdjustLimitation(t *testing.T) {
	limiter := NewHillClimbing().(*HillClimbing)
	acquired, err := limiter.Acquire()
	require.NoError(t, err)
	updater, ok := acquired.(*HillClimbingUpdater)
	require.True(t, ok)

	// Simulate a scenario where the limiter is set to extend its capacity
	err = updater.adjustLimitation(HillClimbingOptionExtend)
	require.NoError(t, err)
	assert.Greater(t, limiter.limitation.Load(), initialLimitation)

	// Simulate a scenario where the limiter is set to shrink its capacity
	err = updater.adjustLimitation(HillClimbingOptionShrink)
	require.NoError(t, err)
	assert.Less(t, limiter.limitation.Load(), initialLimitation)
}

// Test HillClimbing's Remaining capacity behavior
func TestHillClimbing_Remaining(t *testing.T) {
	limiter := NewHillClimbing().(*HillClimbing)
	limiter.limitation.Store(100)
	limiter.inflight.Store(30)

	remaining := limiter.Remaining()
	assert.Equal(t, uint64(70), remaining)

	// Simulate that inflight requests exceed the limitation
	limiter.inflight.Store(120)
	remaining = limiter.Remaining()
	assert.Equal(t, uint64(0), remaining)
}

// TestHillClimbing_Acquire_ConcurrentCapacity verifies that concurrent Acquire calls
// never admit more than the limitation while slots are held: every observation of
// Inflight must stay within the limitation.
func TestHillClimbing_Acquire_ConcurrentCapacity(t *testing.T) {
	const (
		limitation = 16
		workers    = 64
		iterations = 5000
	)

	limiter := NewHillClimbing().(*HillClimbing)
	limiter.limitation.Store(limitation)
	// freeze the rounds so DoUpdate never adjusts the limitation during the test
	limiter.lastUpdatedTime.Store(time.Now().Add(time.Hour))

	var (
		violations atomic.Int64
		wg         sync.WaitGroup
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				updater, err := limiter.Acquire()
				if err != nil {
					assert.ErrorIs(t, err, ErrReachLimitation)
					continue
				}
				if limiter.Inflight() > limitation {
					violations.Add(1)
				}
				assert.NoError(t, updater.DoUpdate())
			}
		}()
	}
	wg.Wait()

	assert.Zero(t, violations.Load())
	assert.Equal(t, uint64(0), limiter.Inflight())
}
