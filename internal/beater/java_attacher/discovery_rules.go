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

import (
	"fmt"
	"regexp"
)

type discoveryRule interface {
	include() bool
	match(jvm *jvmDetails) bool
}

type includeAllRule struct{}

func (includeAllRule) match(jvm *jvmDetails) bool {
	return true
}

func (includeAllRule) include() bool {
	return true
}

func (includeAllRule) String() string {
	return "--includeAll"
}

type userDiscoveryRule struct {
	isIncludeRule bool
	user          string
}

func (rule *userDiscoveryRule) match(jvm *jvmDetails) bool {
	return jvm.user == rule.user
}

func (rule *userDiscoveryRule) include() bool {
	return rule.isIncludeRule
}

func (rule *userDiscoveryRule) String() string {
	if rule.isIncludeRule {
		return fmt.Sprintf("--include-user=%v", rule.user)
	}
	return fmt.Sprintf("--exclude-user=%v", rule.user)
}

type cmdLineDiscoveryRule struct {
	argumentName  string
	isIncludeRule bool
	regex         *regexp.Regexp
}

func (rule *cmdLineDiscoveryRule) match(jvm *jvmDetails) bool {
	return rule.regex.MatchString(jvm.cmdLineArgs)
}

func (rule *cmdLineDiscoveryRule) include() bool {
	return rule.isIncludeRule
}

func (rule *cmdLineDiscoveryRule) String() string {
	return fmt.Sprintf("--%v=%v", rule.argumentName, rule.regex.String())
}
