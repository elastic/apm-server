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

package rumv3

import (
	"fmt"
	"sync"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
)

func init() {
	metadataPool.New = func() interface{} {
		return &metadata{}
	}
}

var metadataPool sync.Pool

func fetchMetadata() *metadata {
	return metadataPool.Get().(*metadata)
}
func releaseMetadata(m *metadata) {
	m.Reset()
	metadataPool.Put(m)
}

// DecodeMetadata uses the given decoder to create the input models,
// then runs the defined validations on the input models
// and finally maps the values fom the input model to the given *model.Metadata instance
func DecodeMetadata(d decoder.Decoder, out *model.Metadata) error {
	m := metadataWithKey{Metadata: *fetchMetadata()}
	defer releaseMetadata(&m.Metadata)
	if err := d.Decode(&m); err != nil {
		return err
	}
	if err := m.validate(); err != nil {
		return err
	}
	mapToModel(&m.Metadata, out)
	return nil
}

func mapToModel(m *metadata, out *model.Metadata) {
	// Labels
	out.Labels = common.MapStr{}
	out.Labels.Update(m.Labels)

	// Service
	if m.Service.Agent.Name.IsSet() {
		out.Service.Agent.Name = m.Service.Agent.Name.Val
	}
	if m.Service.Agent.Version.IsSet() {
		out.Service.Agent.Version = m.Service.Agent.Version.Val
	}
	if m.Service.Environment.IsSet() {
		out.Service.Environment = m.Service.Environment.Val
	}
	if m.Service.Framework.Name.IsSet() {
		out.Service.Framework.Name = m.Service.Framework.Name.Val
	}
	if m.Service.Framework.Version.IsSet() {
		out.Service.Framework.Version = m.Service.Framework.Version.Val
	}
	if m.Service.Language.Name.IsSet() {
		out.Service.Language.Name = m.Service.Language.Name.Val
	}
	if m.Service.Language.Version.IsSet() {
		out.Service.Language.Version = m.Service.Language.Version.Val
	}
	if m.Service.Name.IsSet() {
		out.Service.Name = m.Service.Name.Val
	}
	if m.Service.Runtime.Name.IsSet() {
		out.Service.Runtime.Name = m.Service.Runtime.Name.Val
	}
	if m.Service.Runtime.Version.IsSet() {
		out.Service.Runtime.Version = m.Service.Runtime.Version.Val
	}
	if m.Service.Version.IsSet() {
		out.Service.Version = m.Service.Version.Val
	}

	// User
	if m.User.ID.IsSet() {
		out.User.ID = fmt.Sprint(m.User.ID.Val)
	}
	if m.User.Email.IsSet() {
		out.User.Email = m.User.Email.Val
	}
	if m.User.Name.IsSet() {
		out.User.Name = m.User.Name.Val
	}
}
