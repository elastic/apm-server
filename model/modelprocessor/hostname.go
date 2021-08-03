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

package modelprocessor

import (
	"context"

	"github.com/elastic/apm-server/model"
)

// SetHostHostname is a transform.Processor that sets the final
// host.name and host.hostname values, according to whether the
// event originated from within Kubernetes or not.
type SetHostHostname struct{}

// ProcessBatch sets or overrides the host.name and host.hostname fields for events.
func (SetHostHostname) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for i := range *b {
		setHostHostname(&(*b)[i])
	}
	return nil
}

func setHostHostname(event *model.APMEvent) {
	switch {
	case event.Kubernetes.NodeName != "":
		// host.kubernetes.node.name is set: set host.hostname to its value.
		event.Host.Hostname = event.Kubernetes.NodeName
	case event.Kubernetes.PodName != "" || event.Kubernetes.PodUID != "" || event.Kubernetes.Namespace != "":
		// kubernetes.* is set, but kubernetes.node.name is not: don't set host.hostname at all.
		event.Host.Hostname = ""
	default:
		// Otherwise use the originally specified host.hostname value.
	}
	if event.Host.Name == "" {
		event.Host.Name = event.Host.Hostname
	}
}
