// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"encoding/json"

	// NOTE(axw) encoding/json is faster for encoding,
	// json-iterator is faster for decoding.
	jsoniter "github.com/json-iterator/go"

	"github.com/elastic/apm-server/model"
)

// JSONCodec is an implementation of Codec, using JSON encoding.
type JSONCodec struct{}

// DecodeEvent decodes data as JSON into event.
func (JSONCodec) DecodeEvent(data []byte, event *model.APMEvent) error {
	return jsoniter.ConfigFastest.Unmarshal(data, event)
}

// EncodeEvent encodes event as JSON.
func (JSONCodec) EncodeEvent(event *model.APMEvent) ([]byte, error) {
	return json.Marshal(event)
}
