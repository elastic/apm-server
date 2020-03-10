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

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/fmtstr"
	libilm "github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

const pattern = "000001"

// MakeDefaultSupporter creates the ILM supporter for APM that is passed to libbeat.
func MakeDefaultSupporter(
	log *logp.Logger,
	info beat.Info,
	mode libilm.Mode,
	ilmConfig Config,
	eventIndexNames map[string]string) ([]libilm.Supporter, error) {

	if log == nil {
		log = logp.NewLogger(logs.Ilm)
	} else {
		log = log.Named(logs.Ilm)
	}

	var supporters []libilm.Supporter

	for _, p := range ilmConfig.Setup.Policies {
		index, ok := eventIndexNames[p.EventType]
		if !ok {
			return nil, errors.Errorf("index name missing for event %s when building ILM supporter", p.EventType)
		}
		alias, err := applyStaticFmtstr(info, index)
		if err != nil {
			return nil, err
		}

		supporter := libilm.NewStdSupport(log, mode, libilm.Alias{Name: alias, Pattern: pattern},
			libilm.Policy{Name: p.Name, Body: p.Policy}, ilmConfig.Setup.Overwrite, true)
		supporters = append(supporters, supporter)
	}
	return supporters, nil
}

func applyStaticFmtstr(info beat.Info, s string) (string, error) {
	fmt, err := fmtstr.CompileEvent(s)
	if err != nil {
		return "", err
	}
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
