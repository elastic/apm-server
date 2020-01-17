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

package beater

import (
	"context"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/beater/config"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func notifyListening(ctx context.Context, config *config.Config, reporter publish.Reporter) {
	logp.NewLogger(logs.Onboarding).Info("Publishing onboarding document")
	reporter(ctx, publish.PendingReq{
		Transformables: []transform.Transformable{onboardingDoc{listenAddr: config.Host}},
		Tcontext:       &transform.Context{RequestTime: time.Now()},
	})
}

type onboardingDoc struct {
	listenAddr string
}

func (o onboardingDoc) Transform(tctx *transform.Context) []beat.Event {
	return []beat.Event{{
		Timestamp: tctx.RequestTime,
		Fields: common.MapStr{
			"processor": common.MapStr{"name": "onboarding", "event": "onboarding"},
			"observer":  common.MapStr{"listening": o.listenAddr},
		},
	}}
}
