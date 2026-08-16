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
	"fmt"
	"strings"
)

const (
	ProviderFilterKey = "padasvc"
	ClusterKey        = "adaptiveService"
	LoadBalanceKeyP2C = "p2c"

	EnabledKey   = "adaptive-service.enabled"
	EnabledValue = "1"
	UpdaterKey   = "adaptive-service.updater"
	RemainingKey = "adaptive-service.remaining"
	InflightKey  = "adaptive-service.inflight"

	HillClimbingMetricKey = "hill-climbing"

	AdaptiveServiceInterruptedMessage = "adaptive service interrupted"
	ReachLimitationMessage            = "reach limitation"
)

var ReachLimitationErrorString = fmt.Sprintf("%s: %s",
	AdaptiveServiceInterruptedMessage, ReachLimitationMessage)

func DoesAdaptiveServiceReachLimitation(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == ReachLimitationErrorString
}

func IsAdaptiveServiceFailed(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), AdaptiveServiceInterruptedMessage)
}
