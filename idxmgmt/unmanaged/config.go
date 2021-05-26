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

package unmanaged

import (
	"fmt"

	libcommon "github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/idxmgmt/common"
)

type Config struct {
	Index   string            `config:"index"`
	Indices *libcommon.Config `config:"indices"`
}

func (cfg *Config) Customized() bool {
	if cfg == nil {
		return false
	}
	return cfg.Index != "" || cfg.Indices != nil
}

func (cfg *Config) SelectorConfig() (*libcommon.Config, error) {
	var idcsCfg = libcommon.NewConfig()

	// set defaults
	if cfg.Index == "" {
		// set fallback default index
		idcsCfg.SetString("index", -1, common.FallbackIndex)

		// set default indices if not set
		if cfg.Indices == nil {
			indicesCfg, err := libcommon.NewConfigFrom(conditionalIndices())
			if err != nil {
				return nil, err
			}
			idcsCfg.SetChild("indices", -1, indicesCfg)
		}
	}

	// use custom config settings where available
	if cfg.Index != "" {
		if err := idcsCfg.SetString("index", -1, cfg.Index); err != nil {
			return nil, err
		}
	}
	if cfg.Indices != nil {
		if err := idcsCfg.SetChild("indices", -1, cfg.Indices); err != nil {
			return nil, err
		}
	}
	return idcsCfg, nil
}

func conditionalIndices() []map[string]interface{} {
	conditions := []map[string]interface{}{
		common.ConditionalOnboardingIndex(),
		common.ConditionalSourcemapIndex(),
	}
	for _, k := range common.EventTypes {
		idxStr := fmt.Sprintf("%s-%s%s", common.APMPrefix, k, "-%{+yyyy.MM.dd}")
		conditions = append(conditions, common.Condition(k, idxStr))
	}
	return conditions
}
