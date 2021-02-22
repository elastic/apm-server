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

package modelprocessor

import (
	"context"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/transform"
)

// SetDefaultServiceEnvironment is a transform.Processor that sets a default
// service.environment value for events without one already set.
type SetDefaultServiceEnvironment struct {
	// DefaultServiceEnvironment is the default service.environment value
	// to set for events without one already set.
	DefaultServiceEnvironment string
}

// ProcessTransformables sets a default service.value for events without one already set.
func (s *SetDefaultServiceEnvironment) ProcessTransformables(ctx context.Context, in []transform.Transformable) ([]transform.Transformable, error) {
	for _, t := range in {
		switch t := t.(type) {
		case *model.Transaction:
			if t.Metadata.Service.Environment == "" {
				t.Metadata.Service.Environment = s.DefaultServiceEnvironment
			}
		case *model.Span:
			if t.Metadata.Service.Environment == "" {
				t.Metadata.Service.Environment = s.DefaultServiceEnvironment
			}
		case *model.Metricset:
			if t.Metadata.Service.Environment == "" {
				t.Metadata.Service.Environment = s.DefaultServiceEnvironment
			}
		case *model.Error:
			if t.Metadata.Service.Environment == "" {
				t.Metadata.Service.Environment = s.DefaultServiceEnvironment
			}
		}
	}
	return in, nil
}
