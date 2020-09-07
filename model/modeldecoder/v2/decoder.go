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

package v2

import (
	"fmt"
	"net"
	"sync"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
)

func init() {
	metadataRootPool.New = func() interface{} {
		return &metadataRoot{}
	}
	// metadataNoKeyPool.New = func() interface{} {
	// 	return &metadataNoKey{}
	// }
}

var metadataRootPool sync.Pool

func fetchMetadataRoot() *metadataRoot {
	return metadataRootPool.Get().(*metadataRoot)
}
func releaseMetadataRoot(m *metadataRoot) {
	m.Reset()
	metadataRootPool.Put(m)
}

// DecodeMetadata uses the given decoder to create the input models,
// then runs the defined validations on the input models
// and finally maps the values fom the input model to the given *model.Metadata instance
//
// DecodeMetadata should be used when the underlying byte stream does not contain the
// `metadata` key, but only the metadata.
func DecodeMetadata(d decoder.Decoder, out *model.Metadata) error {
	return decode(func(m *metadataRoot) error {
		return d.Decode(&m.Metadata)
	}, out)
}

// DecodeNestedMetadata uses the given decoder to create the input models,
// then runs the defined validations on the input models
// and finally maps the values fom the input model to the given *model.Metadata instance
//
// DecodeNestedMetadata should be used when the underlying byte stream does start with the `metadata` key
func DecodeNestedMetadata(d decoder.Decoder, out *model.Metadata) error {
	return decode(func(m *metadataRoot) error {
		return d.Decode(m)
	}, out)
}

func decode(decoderFn func(m *metadataRoot) error, out *model.Metadata) error {
	m := fetchMetadataRoot()
	defer releaseMetadataRoot(m)
	if err := decoderFn(m); err != nil {
		return fmt.Errorf("decode error %w", err)
	}
	if err := m.validate(); err != nil {
		return fmt.Errorf("validation error %w", err)
	}
	mapToMetadataModel(&m.Metadata, out)
	return nil
}

func mapToMetadataModel(m *metadata, out *model.Metadata) {
	// Cloud
	if !m.Cloud.Account.ID.IsNil() {
		out.Cloud.AccountID = m.Cloud.Account.ID.Val
	}
	if !m.Cloud.Account.Name.IsNil() {
		out.Cloud.AccountName = m.Cloud.Account.Name.Val
	}
	if !m.Cloud.AvailabilityZone.IsNil() {
		out.Cloud.AvailabilityZone = m.Cloud.AvailabilityZone.Val
	}
	if !m.Cloud.Instance.ID.IsNil() {
		out.Cloud.InstanceID = m.Cloud.Instance.ID.Val
	}
	if !m.Cloud.Instance.Name.IsNil() {
		out.Cloud.InstanceName = m.Cloud.Instance.Name.Val
	}
	if !m.Cloud.Machine.Type.IsNil() {
		out.Cloud.MachineType = m.Cloud.Machine.Type.Val
	}
	if !m.Cloud.Project.ID.IsNil() {
		out.Cloud.ProjectID = m.Cloud.Project.ID.Val
	}
	if !m.Cloud.Project.Name.IsNil() {
		out.Cloud.ProjectName = m.Cloud.Project.Name.Val
	}
	if !m.Cloud.Provider.IsNil() {
		out.Cloud.Provider = m.Cloud.Provider.Val
	}
	if !m.Cloud.Region.IsNil() {
		out.Cloud.Region = m.Cloud.Region.Val
	}

	// Labels
	if len(m.Labels) > 0 {
		out.Labels = common.MapStr{}
		out.Labels.Update(m.Labels)
	}

	// Process
	if len(m.Process.Argv) > 0 {
		out.Process.Argv = m.Process.Argv
	}
	if !m.Process.Pid.IsNil() {
		out.Process.Pid = m.Process.Pid.Val
	}
	if !m.Process.Ppid.IsNil() {
		var pid = m.Process.Ppid.Val
		out.Process.Ppid = &pid
	}
	if !m.Process.Title.IsNil() {
		out.Process.Title = m.Process.Title.Val
	}

	// Service
	if !m.Service.Agent.EphemeralID.IsNil() {
		out.Service.Agent.EphemeralID = m.Service.Agent.EphemeralID.Val
	}
	if !m.Service.Agent.Name.IsNil() {
		out.Service.Agent.Name = m.Service.Agent.Name.Val
	}
	if !m.Service.Agent.Version.IsNil() {
		out.Service.Agent.Version = m.Service.Agent.Version.Val
	}
	if !m.Service.Environment.IsNil() {
		out.Service.Environment = m.Service.Environment.Val
	}
	if !m.Service.Framework.Name.IsNil() {
		out.Service.Framework.Name = m.Service.Framework.Name.Val
	}
	if !m.Service.Framework.Version.IsNil() {
		out.Service.Framework.Version = m.Service.Framework.Version.Val
	}
	if !m.Service.Language.Name.IsNil() {
		out.Service.Language.Name = m.Service.Language.Name.Val
	}
	if !m.Service.Language.Version.IsNil() {
		out.Service.Language.Version = m.Service.Language.Version.Val
	}
	if !m.Service.Name.IsNil() {
		out.Service.Name = m.Service.Name.Val
	}
	if !m.Service.Node.Name.IsNil() {
		out.Service.Node.Name = m.Service.Node.Name.Val
	}
	if !m.Service.Runtime.Name.IsNil() {
		out.Service.Runtime.Name = m.Service.Runtime.Name.Val
	}
	if !m.Service.Runtime.Version.IsNil() {
		out.Service.Runtime.Version = m.Service.Runtime.Version.Val
	}
	if !m.Service.Version.IsNil() {
		out.Service.Version = m.Service.Version.Val
	}

	// System
	if !m.System.Architecture.IsNil() {
		out.System.Architecture = m.System.Architecture.Val
	}
	if !m.System.ConfiguredHostname.IsNil() {
		out.System.ConfiguredHostname = m.System.ConfiguredHostname.Val
	}
	if !m.System.Container.ID.IsNil() {
		out.System.Container.ID = m.System.Container.ID.Val
	}
	if !m.System.DetectedHostname.IsNil() {
		out.System.DetectedHostname = m.System.DetectedHostname.Val
	}
	if m.System.ConfiguredHostname.IsNil() && m.System.DetectedHostname.IsNil() {
		out.System.DetectedHostname = m.System.HostnameDeprecated.Val
	}
	if !m.System.IP.IsNil() {
		out.System.IP = net.ParseIP(m.System.IP.Val)
	}
	if !m.System.Kubernetes.Namespace.IsNil() {
		out.System.Kubernetes.Namespace = m.System.Kubernetes.Namespace.Val
	}
	if !m.System.Kubernetes.Node.Name.IsNil() {
		out.System.Kubernetes.NodeName = m.System.Kubernetes.Node.Name.Val
	}
	if !m.System.Kubernetes.Pod.Name.IsNil() {
		out.System.Kubernetes.PodName = m.System.Kubernetes.Pod.Name.Val
	}
	if !m.System.Kubernetes.Pod.UID.IsNil() {
		out.System.Kubernetes.PodUID = m.System.Kubernetes.Pod.UID.Val
	}
	if !m.System.Platform.IsNil() {
		out.System.Platform = m.System.Platform.Val
	}

	// User
	if !m.User.ID.IsNil() {
		out.User.ID = fmt.Sprint(m.User.ID.Val)
	}
	if !m.User.Email.IsNil() {
		out.User.Email = m.User.Email.Val
	}
	if !m.User.Name.IsNil() {
		out.User.Name = m.User.Name.Val
	}
}
