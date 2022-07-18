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

package javaattacher

import "regexp"

type discoveryRule interface {
	include() bool
	match(jvm *JvmDetails) bool
}

type includeAllRule struct{}

func (includeAllRule) match(jvm *JvmDetails) bool {
	return true
}

func (includeAllRule) include() bool {
	return true
}

type userDiscoveryRule struct {
	isIncludeRule bool
	user          string
}

func (rule userDiscoveryRule) match(jvm *JvmDetails) bool {
	return jvm.user == rule.user
}

func (rule userDiscoveryRule) include() bool {
	return rule.isIncludeRule
}

type cmdLineDiscoveryRule struct {
	isIncludeRule bool
	regex         *regexp.Regexp
}

func (rule cmdLineDiscoveryRule) match(jvm *JvmDetails) bool {
	return rule.regex.MatchString(jvm.cmdLineArgs)
}

func (rule cmdLineDiscoveryRule) include() bool {
	return rule.isIncludeRule
}
