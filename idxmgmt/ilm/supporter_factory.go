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

package ilm

import (
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/fmtstr"
	libilm "github.com/elastic/beats/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

// MakeDefaultSupporter creates the ILM supporter for APM that is passed to libbeat.
func MakeDefaultSupporter(log *logp.Logger, info beat.Info, cfg *common.Config) (libilm.Supporter, error) {
	if log == nil {
		log = logp.NewLogger(logs.Ilm)
	} else {
		log = log.Named(logs.Ilm)
	}

	var ilmCfg Config
	if err := cfg.Unpack(&ilmCfg); err != nil {
		return nil, err
	}
	if ilmCfg.AliasName == nil || ilmCfg.PolicyName == nil {
		return nil, errors.New("ilm alias and policy must be configured")
	}
	aliasName, err := applyStaticFmtstr(info, ilmCfg.AliasName)
	if err != nil {
		return nil, err
	}

	policyName, err := applyStaticFmtstr(info, ilmCfg.PolicyName)
	if err != nil {
		return nil, err
	}

	p, ok := eventPolicies[ilmCfg.Event]
	if !ok {
		return nil, errors.Errorf("policy for %s undefined", ilmCfg.Event)
	}
	policy := libilm.Policy{
		Name: policyName,
		Body: p,
	}
	alias := libilm.Alias{
		Name:    aliasName,
		Pattern: pattern,
	}

	return libilm.NewStdSupport(log, libilm.ModeEnabled, alias, policy, false, true), nil
}

func applyStaticFmtstr(info beat.Info, fmt *fmtstr.EventFormatString) (string, error) {
	return fmt.Run(&beat.Event{
		Fields: common.MapStr{
			// beat object was left in for backward compatibility reason for older configs.
			"beat": common.MapStr{
				"name":    info.Beat,
				"version": info.Version,
			},
			"observer": common.MapStr{
				"name":    info.Beat,
				"version": info.Version,
			},
		},
		Timestamp: time.Now(),
	})
}
