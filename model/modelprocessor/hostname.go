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

// SetSystemHostname is a transform.Processor that sets the final
// host.name and host.hostname values, according to whether the
// system is running in Kubernetes or not.
type SetSystemHostname struct{}

// ProcessBatch sets or overrides the host.name and host.hostname fields for events.
func (SetSystemHostname) ProcessBatch(ctx context.Context, b *model.Batch) error {
	return MetadataProcessorFunc(setSystemHostname).ProcessBatch(ctx, b)
}

func setSystemHostname(ctx context.Context, meta *model.Metadata) error {
	switch {
	case meta.Kubernetes.NodeName != "":
		// system.kubernetes.node.name is set: set host.hostname to its value.
		meta.System.DetectedHostname = meta.Kubernetes.NodeName
	case meta.Kubernetes.PodName != "" || meta.Kubernetes.PodUID != "" || meta.Kubernetes.Namespace != "":
		// kubernetes.* is set, but kubernetes.node.name is not: don't set host.hostname at all.
		meta.System.DetectedHostname = ""
	default:
		// Otherwise use the originally specified host.hostname value.
	}
	if meta.System.ConfiguredHostname == "" {
		meta.System.ConfiguredHostname = meta.System.DetectedHostname
	}
	return nil
}
