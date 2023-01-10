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

package agentcfg

import (
	"context"
	"strings"
)

type SanitizingFetcher struct {
	Fetcher Fetcher
}

// Fetch calls f.Fetcher.Fetch, and then sanitizes the result for untrusted agents (RUM, iOS).
func (f SanitizingFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	result, err := f.Fetcher.Fetch(ctx, query)
	if err != nil {
		return Result{}, err
	}
	return sanitize(query.InsecureAgents, result), nil
}

func sanitize(insecureAgents []string, result Result) Result {
	if len(insecureAgents) == 0 {
		return result
	}
	hasDataForAgent := containsAnyPrefix(result.Source.Agent, insecureAgents) || result.Source.Agent == ""
	if !hasDataForAgent {
		return zeroResult()
	}
	settings := Settings{}
	for k, v := range result.Source.Settings {
		if UnrestrictedSettings[k] {
			settings[k] = v
		}
	}
	return Result{Source: Source{Etag: result.Source.Etag, Settings: settings}}
}

func containsAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
