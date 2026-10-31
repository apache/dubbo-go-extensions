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
	"dubbo.apache.org/dubbo-go/v3/client"
)

import (
	_ "github.com/apache/dubbo-go-extensions/cluster/loadbalance/p2c"
)

// WithClientAdaptiveService selects the adaptive cluster and P2C load balancer
// for all references created by the client. A reference-level option can override either.
func WithClientAdaptiveService() client.ClientOption {
	return func(opts *client.ClientOptions) {
		client.WithClientClusterAdaptiveService()(opts)
		client.WithClientLoadBalanceP2C()(opts)
	}
}
