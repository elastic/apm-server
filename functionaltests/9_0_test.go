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

package functionaltests

import (
	"testing"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

func TestUpgrade_8_18_to_9_0(t *testing.T) {
	t.Parallel()

	runBasicUpgradeSnapshotTest(
		t,
		"8.18",
		"9.0",
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			// TODO: remove once fixed
			populateSourcemapFetcher403,
		},
	)
}
