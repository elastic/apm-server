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

package modelprocessor_test

import (
	"testing"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
)

func TestSetServiceNodeName(t *testing.T) {
	withServiceNodeName := model.Metadata{
		Service: model.Service{
			Node: model.ServiceNode{
				Name: "node_name",
			},
		},
	}
	withConfiguredHostname := model.Metadata{
		System: model.System{
			ConfiguredHostname: "configured_hostname",
		},
	}
	withContainerID := withConfiguredHostname
	withContainerID.Container.ID = "container_id"

	processor := modelprocessor.SetServiceNodeName{}

	testProcessBatchMetadata(t, processor, withServiceNodeName, withServiceNodeName) // unchanged
	testProcessBatchMetadata(t, processor, withConfiguredHostname,
		metadataWithServiceNodeName(withConfiguredHostname, "configured_hostname"),
	)
	testProcessBatchMetadata(t, processor, withContainerID,
		metadataWithServiceNodeName(withContainerID, "container_id"),
	)
}

func metadataWithServiceNodeName(in model.Metadata, nodeName string) model.Metadata {
	in.Service.Node.Name = nodeName
	return in
}
