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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/apm-server/internal/beater/otlp"
)

func TestSetClientMetadata(t *testing.T) {
	modelpbIP1234 := modelpb.MustParseIP("1.2.3.4")
	netIP1234 := net.ParseIP("1.2.3.4")
	netipAddr5678 := netip.MustParseAddr("5.6.7.8")
	netipAddr10 := netip.MustParseAddr("10.10.10.10")

	for _, test := range []struct {
		ctx      context.Context
		in       *modelpb.APMEvent
		expected *modelpb.APMEvent
	}{{
		ctx: context.Background(),
		in: &modelpb.APMEvent{
			Client: &modelpb.Client{Ip: modelpbIP1234},
		},
		expected: &modelpb.APMEvent{
			Client: &modelpb.Client{Ip: modelpbIP1234},
		},
	}, {
		ctx: context.Background(),
		in: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "iOS/swift"},
			Client: &modelpb.Client{Ip: modelpbIP1234},
		},
		expected: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "iOS/swift"},
			Client: &modelpb.Client{Ip: modelpbIP1234},
		},
	}, {
		ctx: context.Background(),
		in: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "iOS/swift"},
		},
		expected: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "iOS/swift"},
		},
	}, {
		ctx: context.Background(),
		in: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "android/java"},
			Client: &modelpb.Client{Ip: modelpbIP1234},
		},
		expected: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "android/java"},
			Client: &modelpb.Client{Ip: modelpbIP1234},
		},
	}, {
		ctx: context.Background(),
		in: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "android/java"},
		},
		expected: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "android/java"},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr: &net.TCPAddr{IP: netIP1234, Port: 4321},
			ClientIP:   netipAddr5678,
		}),
		in: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "iOS/swift"},
		},
		expected: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "iOS/swift"},
			Client: &modelpb.Client{Ip: modelpb.Addr2IP(netipAddr5678)},
			Source: &modelpb.Source{Ip: modelpbIP1234, Port: 4321},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr:  &net.TCPAddr{IP: netIP1234, Port: 4321},
			SourceNATIP: netipAddr10,
			ClientIP:    netipAddr5678,
		}),
		in: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "iOS/swift"},
		},
		expected: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "iOS/swift"},
			Client: &modelpb.Client{Ip: modelpb.Addr2IP(netipAddr5678)},
			Source: &modelpb.Source{
				Ip:   modelpbIP1234,
				Port: 4321,
				Nat:  &modelpb.NAT{Ip: modelpb.Addr2IP(netipAddr10)},
			},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr: &net.TCPAddr{IP: netIP1234, Port: 4321},
			ClientIP:   netipAddr5678,
		}),
		in: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "android/java"},
		},
		expected: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "android/java"},
			Client: &modelpb.Client{Ip: modelpb.Addr2IP(netipAddr5678)},
			Source: &modelpb.Source{Ip: modelpbIP1234, Port: 4321},
		},
	}, {
		ctx: interceptors.ContextWithClientMetadata(context.Background(), interceptors.ClientMetadataValues{
			SourceAddr:  &net.TCPAddr{IP: netIP1234, Port: 4321},
			SourceNATIP: netipAddr10,
			ClientIP:    netipAddr5678,
		}),
		in: &modelpb.APMEvent{
			Agent: &modelpb.Agent{Name: "android/java"},
		},
		expected: &modelpb.APMEvent{
			Agent:  &modelpb.Agent{Name: "android/java"},
			Client: &modelpb.Client{Ip: modelpb.Addr2IP(netipAddr5678)},
			Source: &modelpb.Source{
				Ip:   modelpbIP1234,
				Port: 4321,
				Nat:  &modelpb.NAT{Ip: modelpb.Addr2IP(netipAddr10)},
			},
		},
	}} {
		batch := modelpb.Batch{test.in}
		err := otlp.SetClientMetadata(test.ctx, &batch)
		assert.NoError(t, err)
		assert.Empty(t, cmp.Diff(test.expected, batch[0], protocmp.Transform()))
	}
}
