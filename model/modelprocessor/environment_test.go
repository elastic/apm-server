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

package modelprocessor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/transform"
)

func TestSetDefaultServiceEnvironment(t *testing.T) {
	nonEmptyMetadata := model.Metadata{Service: model.Service{Environment: "nonempty"}}
	in := []transform.Transformable{
		// Should be left alone.
		&model.Transaction{Metadata: nonEmptyMetadata},
		&model.Span{Metadata: nonEmptyMetadata},
		&model.Metricset{Metadata: nonEmptyMetadata},
		&model.Error{Metadata: nonEmptyMetadata},
		&model.PprofProfile{Metadata: nonEmptyMetadata},

		// Should be updated.
		&model.Transaction{},
		&model.Span{},
		&model.Metricset{},
		&model.Error{},
		&model.PprofProfile{},
	}

	processor := modelprocessor.SetDefaultServiceEnvironment{
		DefaultServiceEnvironment: "default",
	}
	out, err := processor.ProcessTransformables(context.Background(), in)
	assert.NoError(t, err)

	defaultMetadata := model.Metadata{Service: model.Service{Environment: "default"}}
	assert.Equal(t, []transform.Transformable{
		// Should be left alone.
		&model.Transaction{Metadata: nonEmptyMetadata},
		&model.Span{Metadata: nonEmptyMetadata},
		&model.Metricset{Metadata: nonEmptyMetadata},
		&model.Error{Metadata: nonEmptyMetadata},
		&model.PprofProfile{Metadata: nonEmptyMetadata},

		// Should be updated.
		&model.Transaction{Metadata: defaultMetadata},
		&model.Span{Metadata: defaultMetadata},
		&model.Metricset{Metadata: defaultMetadata},
		&model.Error{Metadata: defaultMetadata},
		&model.PprofProfile{Metadata: defaultMetadata},
	}, out)
}
