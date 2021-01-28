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

// This file creates dependencies on build/test tools, so we can
// track them in go.mod. See:
//     https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

//+build tools

package main

import (
	_ "github.com/jstemmer/go-junit-report"
	_ "github.com/reviewdog/reviewdog"
	_ "github.com/t-yuki/gocover-cobertura"

	_ "github.com/elastic/go-licenser"
	_ "go.elastic.co/go-licence-detector"
)
