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
	"expvar"
	"net/http"
	"sync"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/libbeat/logp"
)

type contextPool struct {
	p sync.Pool
}

func newContextPool() *contextPool {
	pool := contextPool{}
	pool.p.New = func() interface{} {
		return &request.Context{}
	}
	return &pool
}

func (pool *contextPool) handler(h Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := pool.p.Get().(*request.Context)
		defer pool.p.Put(c)
		c.Reset(w, r)

		logHandler(panicHandler(h))(c)
	})
}

func newMuxer(beaterConfig *Config, report publish.Reporter) (*http.ServeMux, error) {
	pool := newContextPool()
	mux := http.NewServeMux()
	logger := logp.NewLogger(logs.Handler)

	for path, route := range AssetRoutes {
		logger.Infof("Path %s added to request handler", path)
		handler, err := route.Handler(route.Processor, beaterConfig, report)
		if err != nil {
			return nil, err
		}
		mux.Handle(path, pool.handler(handler))
	}
	for path, route := range IntakeRoutes {
		logger.Infof("Path %s added to request handler", path)

		handler, err := route.Handler(path, beaterConfig, report)
		if err != nil {
			return nil, err
		}
		mux.Handle(path, pool.handler(handler))
	}

	var kbClient kibana.Client
	if beaterConfig.Kibana.Enabled() {
		kbClient = kibana.NewConnectingClient(beaterConfig.Kibana)
	}
	mux.Handle(agentConfigURL, pool.handler(agentConfigHandler(kbClient, beaterConfig.AgentConfig, beaterConfig.SecretToken)))
	logger.Infof("Path %s added to request handler", agentConfigURL)

	mux.Handle(rootURL, pool.handler(rootHandler(beaterConfig.SecretToken)))

	if beaterConfig.Expvar.isEnabled() {
		path := beaterConfig.Expvar.Url
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, expvar.Handler())
	}
	return mux, nil
}
