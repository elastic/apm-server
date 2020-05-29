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

package common

const APMPrefix = "apm-%{[observer.version]}"

var (
	EventTypes    = []string{"span", "transaction", "error", "metric", "profile"}
	FallbackIndex = "apm-%{[observer.version]}-%{+yyyy.MM.dd}"
)

func ConditionalSourcemapIndex() map[string]interface{} {
	return Condition("sourcemap", APMPrefix+"-sourcemap")
}

func ConditionalOnboardingIndex() map[string]interface{} {
	return Condition("onboarding", APMPrefix+"-onboarding-%{+yyyy.MM.dd}")
}

func Condition(event string, index string) map[string]interface{} {
	return map[string]interface{}{
		"index": index,
		"when":  map[string]interface{}{"contains": map[string]interface{}{"processor.event": event}},
	}
}
