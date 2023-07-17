// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"github.com/elastic/apm-data/model/modelpb"
)

// ProtobufCodec is an implementation of Codec, using protobuf encoding.
type ProtobufCodec struct{}

// DecodeEvent decodes data as protobuf into event.
func (ProtobufCodec) DecodeEvent(data []byte, event *modelpb.APMEvent) error {
	return event.UnmarshalVT(data)
}

// EncodeEvent encodes event as protobuf.
func (ProtobufCodec) EncodeEvent(event *modelpb.APMEvent) ([]byte, error) {
	return event.MarshalVT()
}
