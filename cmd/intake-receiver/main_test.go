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

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadata(t *testing.T) {
	testCase := func(t *testing.T, in, name, version string) {
		var m metadata
		json.Unmarshal([]byte(in), &m)
		var gotName, gotVersion string

		if m.V2.Service.Agent != nil {
			gotName = m.V2.Service.Agent.Name
			gotVersion = m.V2.Service.Agent.Version
		} else {
			gotName = m.V3RUM.Service.Agent.Name
			gotVersion = m.V3RUM.Service.Agent.Version
		}
		assert.Equal(t, gotName, name)
		assert.Equal(t, gotVersion, version)
	}

	t.Run("rum/v3", func(t *testing.T) {
		in := `{"m": {"se": {"n": "apm-a-rum-test-e2e-general-usecase","ve": "0.0.1","en": "prod","a": {"n": "js-base","ve": "4.8.1"},"ru": {"n": "v8","ve": "8.0"},"la": {"n": "javascript","ve": "6"},"fw": {"n": "angular","ve": "2"}},"u": {"id": 123,"em": "user@email.com","un": "John Doe"},"l": {"testTagKey": "testTagValue"},"n":{"c":{"t":"5G"}}}}`
		testCase(t, in, "js-base", "4.8.1")
	})
	t.Run("intake/v2", func(t *testing.T) {
		in := `{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}`
		testCase(t, in, "elastic-node", "3.14.0")

		in = `{"metadata":{"system":{"architecture":"arm64","hostname":"xyz","platform":"darwin"},"process":{"pid":13328,"argv":["/var/folders/35/r4w8sbqj2md1sg944kpnzyth0000gn/T/go-build968804935/b001/exe/main","-host=localhost:8201"],"ppid":13312,"title":"main"},"service":{"agent":{"name":"go","version":"1.14.0"},"language":{"name":"go","version":"go1.17.7"},"name":"main","runtime":{"name":"gc","version":"go1.17.7"}}}}`
		testCase(t, in, "go", "1.14.0")
	})
}
