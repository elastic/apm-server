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

package config

import "strings"

//Mode enumerates the APM Server env
type Mode uint8

const (
	// ModeProduction is the default mode
	ModeProduction Mode = iota

	// ModeExperimental should only be used in development environments. It allows to circumvent some restrictions
	// on the Intake API for faster development.
	ModeExperimental
)

// Unpack parses the given string into a Mode value
func (m *Mode) Unpack(s string) error {
	if strings.ToLower(s) == "experimental" {
		*m = ModeExperimental
		return nil
	}
	*m = ModeProduction
	return nil
}
