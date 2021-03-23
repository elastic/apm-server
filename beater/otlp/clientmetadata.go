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

	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/model"
)

// SetClientMetadata sets metadata relating to the gRPC client in metadata for
// end-user events, which are assumed to have been sent to the server from the
// user's device.
//
// Client metadata is extracted from ctx, injected by interceptors.ClientMetadata.
func SetClientMetadata(ctx context.Context, meta *model.Metadata) error {
	if meta.Service.Agent.Name != "iOS/swift" {
		// This is not an event from an agent we would consider to be
		// running on an end-user device.
		//
		// TODO(axw) use User-Agent in the check, when we know what we
		// should be looking for?
		return nil
	}
	clientMetadata, ok := interceptors.ClientMetadataFromContext(ctx)
	if ok {
		if meta.Client.IP == nil {
			meta.Client.IP = clientMetadata.SourceIP
		}
	}
	return nil
}
