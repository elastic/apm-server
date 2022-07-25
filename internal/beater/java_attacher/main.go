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

//go:build ignore

package main

import (
	"context"
	"log"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/beater/config"
	javaattacher "github.com/elastic/apm-server/beater/java_attacher"
)

func main() {
	logp.DevelopmentSetup(logp.WithSelectors("*"))
	ja, err := javaattacher.New(config.JavaAttacherConfig{
		Enabled:        true,
		DiscoveryRules: []map[string]string{{"include-vmarg": "elastic.apm.attach=true"}},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := ja.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
