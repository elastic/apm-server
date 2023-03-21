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

package loadgen

import (
	"embed"
	"path/filepath"

	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/systemtest/loadgen/eventhandler"

	"go.elastic.co/apm/v2/transport"
)

// events holds the current stored events.
//
//go:embed events/*.ndjson
var events embed.FS

type EventHandlerParams struct {
	Path    string
	URL     string
	Token   string
	APIKey  string
	Limiter *rate.Limiter
}

// NewEventHandler creates a eventhandler which loads the files matching the
// passed regex.
func NewEventHandler(p EventHandlerParams) (*eventhandler.Handler, error) {
	// We call the HTTPTransport constructor to avoid copying all the config
	// parsing that creates the `*http.Client`.
	t, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{})
	if err != nil {
		return nil, err
	}
	transp := eventhandler.NewTransport(t.Client, p.URL, p.Token, p.APIKey)
	return eventhandler.New(eventhandler.Config{
		Path:      filepath.Join("events", p.Path),
		Transport: transp,
		Storage:   events,
		Limiter:   p.Limiter,
	})
}
