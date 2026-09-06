/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to you under the Apache License, Version 2.0
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

import "strings"

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/server"
)

// WithServerAdaptiveService adds the adaptive service provider filter to the server filter chain.
func WithServerAdaptiveService() server.ServerOption {
	return func(opts *server.ServerOptions) {
		for _, filterName := range strings.Split(opts.Provider.Filter, ",") {
			if strings.TrimSpace(filterName) == constant.AdaptiveServiceProviderFilterKey {
				return
			}
		}

		if opts.Provider.Filter == "" {
			opts.Provider.Filter = constant.AdaptiveServiceProviderFilterKey
			return
		}
		opts.Provider.Filter += "," + constant.AdaptiveServiceProviderFilterKey
	}
}
