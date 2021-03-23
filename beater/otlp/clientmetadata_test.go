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

package otlp_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/model"
)

func TestSetClientMetadata(t *testing.T) {
	ip1234 := net.ParseIP("1.2.3.4")
	ip5678 := net.ParseIP("5.6.7.8")

	for _, test := range []struct {
		ctx        context.Context
		meta       model.Metadata
		expectedIP net.IP
	}{{
		ctx:        context.Background(),
		meta:       model.Metadata{Client: model.Client{IP: ip1234}},
		expectedIP: ip1234,
	}, {
		ctx: context.Background(),
		meta: model.Metadata{
			Service: model.Service{Agent: model.Agent{Name: "iOS/swift"}},
			Client:  model.Client{IP: ip1234},
		},
		expectedIP: ip1234,
	}, {
		ctx:  context.Background(),
		meta: model.Metadata{Service: model.Service{Agent: model.Agent{Name: "iOS/swift"}}},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceIP: ip5678,
		}),
		meta:       model.Metadata{Service: model.Service{Agent: model.Agent{Name: "iOS/swift"}}},
		expectedIP: ip5678,
	}} {
		metaCopy := test.meta
		err := otlp.SetClientMetadata(test.ctx, &metaCopy)
		assert.NoError(t, err)

		test.meta.Client.IP = test.expectedIP
		assert.Equal(t, test.meta, metaCopy)
	}
}
