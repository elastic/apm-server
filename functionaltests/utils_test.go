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
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// ecAPICheck verifies if EC_API_KEY env var is set.
// This is a simple check to alert users if this necessary env var
// is not available.
//
// Functional tests are expected to run Terraform code to operate
// on infrastructure required for each tests and to query Elastic
// Cloud APIs. In both cases a valid API key is required.
func ecAPICheck(t *testing.T) {
	t.Helper()
	require.NotEmpty(t, os.Getenv("EC_API_KEY"), "EC_API_KEY env var not set")
}
