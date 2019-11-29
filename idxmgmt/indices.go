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

package idxmgmt

import (
	"fmt"

	"github.com/elastic/beats/libbeat/common"
)

const (
	apmPrefix  = "apm"
	apmPattern = "apm*"
	apmVersion = "%{[observer.version]}"
	apmSuffix  = "-%{+yyyy.MM.dd}"
)

type esIndexConfig struct {
	Index   string         `config:"index"`
	Indices *common.Config `config:"indices"`
}

func (cfg *esIndexConfig) customized() bool {
	if cfg == nil {
		return false
	}
	return cfg.Index != "" || cfg.Indices != nil
}

var defaultIndex = fmt.Sprintf("%s-%s%s", apmPrefix, apmVersion, apmSuffix)

func idxStr(name string, suffix string) string {
	return fmt.Sprintf("%s-%s-%s%s", apmPrefix, apmVersion, name, suffix)
}

func eventIdxNames(dateSuffix bool) map[string]string {
	var suffix = ""
	if dateSuffix {
		suffix = apmSuffix
	}
	idcs := map[string]string{}
	for _, k := range []string{"span", "transaction", "error", "metric", "profile"} {
		idcs[k] = idxStr(k, suffix)
	}
	return idcs
}

func othersIdxNames() map[string]string {
	return map[string]string{
		"sourcemap":  idxStr("sourcemap", ""),
		"onboarding": idxStr("onboarding", apmSuffix),
	}
}

func indices(cfg *esIndexConfig) (*common.Config, error) {
	var idcsCfg = common.NewConfig()

	// set defaults
	if cfg.Index == "" {
		// set fallback default index
		idcsCfg.SetString("index", -1, defaultIndex)

		// set default indices if not set
		if cfg.Indices == nil {
			if indicesCfg, err := common.NewConfigFrom(indicesConditions(false)); err == nil {
				idcsCfg.SetChild("indices", -1, indicesCfg)
			}
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

func ilmIndices() (*common.Config, error) {
	var idcsCfg = common.NewConfig()

	// set fallback index
	idcsCfg.SetString("index", -1, defaultIndex)

	if indicesCfg, err := common.NewConfigFrom(indicesConditions(true)); err == nil {
		idcsCfg.SetChild("indices", -1, indicesCfg)
	}
	return idcsCfg, nil
}

func indicesConditions(ilm bool) []map[string]interface{} {
	indices := eventIdxNames(!ilm)
	for k, v := range othersIdxNames() {
		indices[k] = v
	}
	var conditions []map[string]interface{}
	for k, v := range indices {
		conditions = append(conditions, condition(k, v))
	}
	return conditions
}

func condition(event string, index string) map[string]interface{} {
	return map[string]interface{}{
		"index": index,
		"when":  map[string]interface{}{"contains": map[string]interface{}{"processor.event": event}},
	}
}
