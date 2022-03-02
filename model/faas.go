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

import "github.com/elastic/beats/v7/libbeat/common"

// FAAS holds information about a function as a service.
type FAAS struct {
	ID               string
	Coldstart        *bool
	Execution        string
	TriggerType      string
	TriggerRequestID string
	Name             string
	Version          string
}

func (f *FAAS) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("id", f.ID)
	fields.maybeSetBool("coldstart", f.Coldstart)
	fields.maybeSetString("execution", f.Execution)
	fields.maybeSetString("trigger.type", f.TriggerType)
	fields.maybeSetString("trigger.request_id", f.TriggerRequestID)
	fields.maybeSetString("name", f.Name)
	fields.maybeSetString("version", f.Version)
	return common.MapStr(fields)
}
