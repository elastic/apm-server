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
	"fmt"
	"os"
	"testing"
)

// ecAPICheck verifies if EC_API_KEY env var is set.
// This is a simple check to alert users if this necessary env var
// is not available.
//
// Functional tests are expected to run Terraform code to operate
// on infrastructure required for each tests and to query Elastic
// Cloud APIs. In both cases a valid API key is required.
func ecAPICheck(t *testing.T) error {
	t.Helper()
	apiKey := os.Getenv("EC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("unable to obtain value from EC_API_KEY environment variable")
	}
	return nil
}
