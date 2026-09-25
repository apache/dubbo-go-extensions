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

import (
	"fmt"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
)

const adaptiveServiceExtensionName = "adaptive-service"

var (
	_ extension.Config = (*Config)(nil)
	_ extension.Option = adaptiveServiceOption{}
)

// Config defines the adaptive service extension configuration.
type Config struct{}

func (c *Config) Prefix() string {
	return adaptiveServiceExtensionName
}

func (c *Config) New() extension.Config {
	return &Config{}
}

func (c *Config) Init(scope extension.Scope) error {
	if scope != extension.ServerScope {
		return fmt.Errorf("adaptive service only supports server scope")
	}
	return nil
}

func (c *Config) FilterNames(scope extension.Scope) []string {
	if scope != extension.ServerScope {
		return nil
	}
	return []string{constant.AdaptiveServiceProviderFilterKey}
}

type adaptiveServiceOption struct{}

func (adaptiveServiceOption) Prefix() string {
	return adaptiveServiceExtensionName
}

func (adaptiveServiceOption) Apply(config extension.Config) error {
	if _, ok := config.(*Config); !ok {
		return fmt.Errorf("adaptive service received unexpected config type %T", config)
	}
	return nil
}

// WithAdaptiveService enables adaptive service for a dubbo-go server.
func WithAdaptiveService() extension.Option {
	return adaptiveServiceOption{}
}
