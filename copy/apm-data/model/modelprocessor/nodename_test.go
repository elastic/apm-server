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

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"google.golang.org/protobuf/proto"
)

func TestSetServiceNodeName(t *testing.T) {
	withServiceNodeName := modelpb.APMEvent{
		Service: &modelpb.Service{
			Node: &modelpb.ServiceNode{
				Name: "node_name",
			},
		},
	}
	withConfiguredHostname := modelpb.APMEvent{
		Host: &modelpb.Host{Name: "configured_hostname"},
	}
	withContainerID := proto.Clone(&withConfiguredHostname).(*modelpb.APMEvent)
	withContainerID.Container = &modelpb.Container{
		Id: "container_id",
	}

	processor := modelprocessor.SetServiceNodeName{}

	testProcessBatch(t, processor, &withServiceNodeName, &withServiceNodeName) // unchanged
	testProcessBatch(t, processor, &withConfiguredHostname,
		eventWithServiceNodeName(&withConfiguredHostname, "configured_hostname"),
	)
	testProcessBatch(t, processor, withContainerID,
		eventWithServiceNodeName(withContainerID, "container_id"),
	)
}

func eventWithServiceNodeName(in *modelpb.APMEvent, nodeName string) *modelpb.APMEvent {
	if in.Service == nil {
		in.Service = &modelpb.Service{}
	}
	if in.Service.Node == nil {
		in.Service.Node = &modelpb.ServiceNode{}
	}
	in.Service.Node.Name = nodeName
	return in
}
