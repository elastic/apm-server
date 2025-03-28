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

package asserts

import (
	"fmt"
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"

	"github.com/stretchr/testify/assert"
)

// actual: the result of the last query
// expected: how many we expect the ingestion to have produced
// previous: any previous docs count to consider
func StandaloneDocsCount(t *testing.T, actual, expected, previous esclient.IndicesDocCount) {
	t.Helper()

	var errs []error
	for ds, a := range actual {
		e, ok := expected[ds]
		if !ok {
			errs = append(errs, fmt.Errorf("unexpected index %s", ds))
			continue
		}
		if e < 0 {
			// signal we want to skip checking doc count for this index
			continue
		}
		p := 0
		if v, ok := previous[ds]; ok {
			p = v
		}
		assert.Equal(t, e, a-p,
			fmt.Sprintf("wrong document count for %s", ds))
	}

	assert.Len(t, errs, 0, "unexpected indices found")
}
