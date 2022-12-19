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
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/apm-server/internal/beater/otlp"
)

func TestSetClientMetadata(t *testing.T) {
	netip1234 := netip.MustParseAddr("1.2.3.4")
	ip1234 := net.ParseIP("1.2.3.4")
	ip5678 := netip.MustParseAddr("5.6.7.8")
	ip10 := netip.MustParseAddr("10.10.10.10")

	for _, test := range []struct {
		ctx      context.Context
		in       model.APMEvent
		expected model.APMEvent
	}{{
		ctx: context.Background(),
		in: model.APMEvent{
			Client: model.Client{IP: netip1234},
		},
		expected: model.APMEvent{
			Client: model.Client{IP: netip1234},
		},
	}, {
		ctx: context.Background(),
		in: model.APMEvent{
			Agent:  model.Agent{Name: "iOS/swift"},
			Client: model.Client{IP: netip1234},
		},
		expected: model.APMEvent{
			Agent:  model.Agent{Name: "iOS/swift"},
			Client: model.Client{IP: netip1234},
		},
	}, {
		ctx: context.Background(),
		in: model.APMEvent{
			Agent: model.Agent{Name: "iOS/swift"},
		},
		expected: model.APMEvent{
			Agent: model.Agent{Name: "iOS/swift"},
		},
	}, {
		ctx: context.Background(),
		in: model.APMEvent{
			Agent:  model.Agent{Name: "android/java"},
			Client: model.Client{IP: netip1234},
		},
		expected: model.APMEvent{
			Agent:  model.Agent{Name: "android/java"},
			Client: model.Client{IP: netip1234},
		},
	}, {
		ctx: context.Background(),
		in: model.APMEvent{
			Agent: model.Agent{Name: "android/java"},
		},
		expected: model.APMEvent{
			Agent: model.Agent{Name: "android/java"},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr: &net.TCPAddr{IP: ip1234, Port: 4321},
			ClientIP:   ip5678,
		}),
		in: model.APMEvent{
			Agent: model.Agent{Name: "iOS/swift"},
		},
		expected: model.APMEvent{
			Agent:  model.Agent{Name: "iOS/swift"},
			Client: model.Client{IP: ip5678},
			Source: model.Source{IP: netip1234, Port: 4321},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr:  &net.TCPAddr{IP: ip1234, Port: 4321},
			SourceNATIP: ip10,
			ClientIP:    ip5678,
		}),
		in: model.APMEvent{
			Agent: model.Agent{Name: "iOS/swift"},
		},
		expected: model.APMEvent{
			Agent:  model.Agent{Name: "iOS/swift"},
			Client: model.Client{IP: ip5678},
			Source: model.Source{
				IP:   netip1234,
				Port: 4321,
				NAT:  &model.NAT{IP: ip10},
			},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr: &net.TCPAddr{IP: ip1234, Port: 4321},
			ClientIP:   ip5678,
		}),
		in: model.APMEvent{
			Agent: model.Agent{Name: "android/java"},
		},
		expected: model.APMEvent{
			Agent:  model.Agent{Name: "android/java"},
			Client: model.Client{IP: ip5678},
			Source: model.Source{IP: netip1234, Port: 4321},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr:  &net.TCPAddr{IP: ip1234, Port: 4321},
			SourceNATIP: ip10,
			ClientIP:    ip5678,
		}),
		in: model.APMEvent{
			Agent: model.Agent{Name: "android/java"},
		},
		expected: model.APMEvent{
			Agent:  model.Agent{Name: "android/java"},
			Client: model.Client{IP: ip5678},
			Source: model.Source{
				IP:   netip1234,
				Port: 4321,
				NAT:  &model.NAT{IP: ip10},
			},
		},
	}} {
		batch := model.Batch{test.in}
		err := otlp.SetClientMetadata(test.ctx, &batch)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, batch[0])
	}
}
