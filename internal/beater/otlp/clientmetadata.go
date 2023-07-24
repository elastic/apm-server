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

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/interceptors"
)

// SetClientMetadata sets metadata relating to the gRPC client in end-user
// events, which are assumed to have been sent to the server from the user's device.
//
// Client metadata is extracted from ctx, injected by interceptors.ClientMetadata.
func SetClientMetadata(ctx context.Context, batch *modelpb.Batch) error {
	for i := range *batch {
		event := (*batch)[i]
		if event.GetAgent().GetName() != "iOS/swift" && event.GetAgent().GetName() != "android/java" {
			// This is not an event from an agent we would consider to be
			// running on an end-user device.
			//
			// TODO(axw) use User-Agent in the check, when we know what we
			// should be looking for?
			continue
		}
		clientMetadata, ok := interceptors.ClientMetadataFromContext(ctx)
		if ok {
			if event.GetSource().GetIp() == nil {
				if tcpAddr, ok := clientMetadata.SourceAddr.(*net.TCPAddr); ok {
					if event.Source == nil {
						event.Source = &modelpb.Source{}
					}
					sourceAddrPort := tcpAddr.AddrPort()
					event.Source.Ip = modelpb.Addr2IP(sourceAddrPort.Addr().Unmap())
					event.Source.Port = uint32(sourceAddrPort.Port())
				}
			}
			if event.GetClient().GetIp() == nil && clientMetadata.ClientIP.IsValid() {
				if event.Client == nil {
					event.Client = &modelpb.Client{}
				}
				event.Client.Ip = modelpb.Addr2IP(clientMetadata.ClientIP)
			}
			if clientMetadata.SourceNATIP.IsValid() {
				event.Source.Nat = &modelpb.NAT{
					Ip: modelpb.Addr2IP(clientMetadata.SourceNATIP),
				}
			}
		}
	}
	return nil
}
