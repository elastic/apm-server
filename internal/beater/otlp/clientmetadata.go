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

package otlp

import (
	"context"
	"net"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/beater/interceptors"
)

// SetClientMetadata sets metadata relating to the gRPC client in end-user
// events, which are assumed to have been sent to the server from the user's device.
//
// Client metadata is extracted from ctx, injected by interceptors.ClientMetadata.
func SetClientMetadata(ctx context.Context, batch *model.Batch) error {
	for i := range *batch {
		event := &(*batch)[i]
		if event.Agent.Name != "iOS/swift" && event.Agent.Name != "android/java" {
			// This is not an event from an agent we would consider to be
			// running on an end-user device.
			//
			// TODO(axw) use User-Agent in the check, when we know what we
			// should be looking for?
			continue
		}
		clientMetadata, ok := interceptors.ClientMetadataFromContext(ctx)
		if ok {
			if !event.Source.IP.IsValid() {
				if tcpAddr, ok := clientMetadata.SourceAddr.(*net.TCPAddr); ok {
					event.Source.IP = tcpAddr.AddrPort().Addr()
					event.Source.Port = tcpAddr.Port
				}
			}
			if !event.Client.IP.IsValid() {
				event.Client.IP = clientMetadata.ClientIP
			}
			if clientMetadata.SourceNATIP.IsValid() {
				event.Source.NAT = &model.NAT{IP: clientMetadata.SourceNATIP}
			}
		}
	}
	return nil
}
