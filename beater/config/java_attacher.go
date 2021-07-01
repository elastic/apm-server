// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package config

import "fmt"

// JavaAttacherConfig holds configuration information for running a java
// attacher jarfile.
type JavaAttacherConfig struct {
	Enabled        bool                `config:"enabled"`
	DiscoveryRules []map[string]string `config:"discovery-rules"`
	Config         map[string]string   `config:"config"`
	JavaBin        string
}

func (j JavaAttacherConfig) setup() error {
	if !j.Enabled {
		return nil
	}
	for _, rule := range j.DiscoveryRules {
		if len(rule) != 1 {
			return fmt.Errorf("unexpected discovery rule format: %v", rule)
		}
		for flag := range rule {
			if _, ok := allowlist[flag]; !ok {
				return fmt.Errorf("unrecognized discovery rule: %s", rule)
			}
		}
	}
	return nil
}

var allowlist = map[string]struct{}{
	"include-all":   {},
	"include-main":  {},
	"include-vmarg": {},
	"include-user":  {},
	"exclude-main":  {},
	"exclude-vmarg": {},
	"exclude-user":  {},
}

func defaultJavaAttacherConfig() JavaAttacherConfig {
	return JavaAttacherConfig{Enabled: false}
}
