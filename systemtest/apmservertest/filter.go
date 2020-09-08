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

package apmservertest

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"io"

	"go.elastic.co/apm/model"
	"go.elastic.co/apm/transport"
	"go.elastic.co/fastjson"
)

// TODO(axw) move EventMetadata and filteringTransport to go.elastic.co/apmtest,
// generalising filteringTransport to work with arbitrary base transports. To do
// that we would need to dynamically check for optional interfaces supported by
// the base transport, and create passthrough methods.

// EventMetadata holds event metadata.
type EventMetadata struct {
	System  *model.System
	Process *model.Process
	Service *model.Service
	Labels  model.IfaceMap
}

// EventMetadata holds event metadata.
type EventMetadataFilter interface {
	FilterEventMetadata(*EventMetadata)
}

type filteringTransport struct {
	*transport.HTTPTransport
	filter EventMetadataFilter
}

// SendStream decodes metadata from reader, passes it through the filters,
// and then sends the modified stream to the underlying transport.
func (t *filteringTransport) SendStream(ctx context.Context, stream io.Reader) error {
	zr, err := zlib.NewReader(stream)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(zr)

	// The first object of any request must be a metadata struct.
	var metadataPayload struct {
		Metadata EventMetadata `json:"metadata"`
	}
	if err := decoder.Decode(&metadataPayload); err != nil {
		return err
	}
	t.filter.FilterEventMetadata(&metadataPayload.Metadata)

	// Re-encode metadata.
	var json fastjson.Writer
	json.RawString(`{"metadata":`)
	json.RawString(`{"system":`)
	metadataPayload.Metadata.System.MarshalFastJSON(&json)
	json.RawString(`,"process":`)
	metadataPayload.Metadata.Process.MarshalFastJSON(&json)
	json.RawString(`,"service":`)
	metadataPayload.Metadata.Service.MarshalFastJSON(&json)
	if len(metadataPayload.Metadata.Labels) > 0 {
		json.RawString(`,"labels":`)
		metadataPayload.Metadata.Labels.MarshalFastJSON(&json)
	}
	json.RawString("}}\n")

	// Copy everything to a new zlib-encoded stream and send.
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write(json.Bytes())
	if _, err := io.Copy(zw, io.MultiReader(decoder.Buffered(), zr)); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return t.HTTPTransport.SendStream(ctx, &buf)
}

type defaultMetadataFilter struct{}

func (defaultMetadataFilter) FilterEventMetadata(m *EventMetadata) {
	m.System.Platform = "minix"
	m.System.Architecture = "i386"
	m.System.Container = nil
	m.System.Kubernetes = nil
	m.System.Hostname = "beowulf"
	m.Process.Pid = 1
	m.Process.Ppid = nil
	m.Service.Agent.Version = "0.0.0"
	m.Service.Language.Version = "2.0"
	m.Service.Runtime.Version = "2.0"
	m.Service.Node = nil
	m.Service.Name = "systemtest"
}
