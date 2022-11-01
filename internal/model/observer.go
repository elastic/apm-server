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

package model

import (
	"github.com/elastic/elastic-agent-libs/mapstr"
)

// Observer describes a special network, security, or application device used to detect,
// observe, or create network, security, or application-related events and metrics.
//
// https://www.elastic.co/guide/en/ecs/current/ecs-observer.html
type Observer struct {
	Hostname string
	Name     string
	Type     string
	Version  string
}

func (o *Observer) Fields() mapstr.M {
	var fields mapStr
	fields.maybeSetString("hostname", o.Hostname)
	fields.maybeSetString("name", o.Name)
	fields.maybeSetString("type", o.Type)
	fields.maybeSetString("version", o.Version)
	return mapstr.M(fields)
}
