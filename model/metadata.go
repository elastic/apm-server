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
	"github.com/elastic/beats/v7/libbeat/common"
)

type Metadata struct {
	Service   Service
	Process   Process
	System    System
	User      User
	UserAgent UserAgent
	Client    Client
	Cloud     Cloud
	Labels    common.MapStr
}

func (m *Metadata) set(fields *mapStr, eventLabels common.MapStr) {
	fields.maybeSetMapStr("service", m.Service.Fields())
	fields.maybeSetMapStr("agent", m.Service.Agent.fields())
	fields.maybeSetMapStr("host", m.System.fields())
	fields.maybeSetMapStr("process", m.Process.fields())
	fields.maybeSetMapStr("user", m.User.fields())
	fields.maybeSetMapStr("client", m.Client.fields())
	fields.maybeSetMapStr("user_agent", m.UserAgent.fields())
	fields.maybeSetMapStr("container", m.System.Container.fields())
	fields.maybeSetMapStr("kubernetes", m.System.Kubernetes.fields())
	fields.maybeSetMapStr("cloud", m.Cloud.fields())
	fields.maybeSetMapStr("network", m.System.Network.fields())
	maybeSetLabels(fields, m.Labels, eventLabels)
}
