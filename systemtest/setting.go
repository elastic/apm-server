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

package systemtest

import (
	"fmt"
	"log"
	"strings"
)

func init() {
	// Proactively test with more strict
	// "ignore_malformed" mode by default.
	for _, t := range []string{
		"traces",
		"metrics",
		"logs-apm.error",
		"logs-apm.app",
	} {
		if err := DisableIgnoreMalformed(t); err != nil {
			log.Fatalf("failed to configure ignore_malformed %v", err)
		}
	}
}

// DisableIgnoreMalformed updates component template index setting
// to disable "ignore_malformed" inside mappings.
func DisableIgnoreMalformed(componentTemplate string) error {
	r, err := Elasticsearch.Cluster.PutComponentTemplate(
		fmt.Sprintf("%s@custom", componentTemplate),
		strings.NewReader(`{"template":{"settings":{"index":{"mapping":{"ignore_malformed":"false"}}}}}`),
	)
	if err != nil {
		return err
	}
	if r.IsError() {
		return fmt.Errorf(`request to update "ignore_malformed":"false" failed for %s`, componentTemplate)
	}
	return nil
}
