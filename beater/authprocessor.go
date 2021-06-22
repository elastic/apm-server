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
	"fmt"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/model"
)

// verifyAuthorizedFor is a model.BatchProcessor that checks authorization
// for the agent and service name in the metadata.
func verifyAuthorizedFor(ctx context.Context, meta *model.Metadata) error {
	result, err := authorization.AuthorizedFor(ctx, authorization.Resource{
		AgentName:   meta.Service.Agent.Name,
		ServiceName: meta.Service.Name,
	})
	if err != nil {
		return err
	}
	if result.Authorized {
		return nil
	}
	return fmt.Errorf("%w: %s", authorization.ErrUnauthorized, result.Reason)
}
