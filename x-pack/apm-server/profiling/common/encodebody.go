// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package common

import (
	"bytes"
	"io"

	"github.com/goccy/go-json"
)

// EncodeBody encodes input to a JSON Reader to be consumed
// by esutil.BulkIndexerItem.Body.
func EncodeBody(input any) (io.ReadSeeker, error) {
	b, err := EncodeBodyBytes(input)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

func EncodeBodyBytes(input any) ([]byte, error) {
	var buf bytes.Buffer
	// esutil.BulkIndexer will keep the byte buffers allocated in this function
	// alive for the entire duration of the associated request-response. Given
	// that we do not know ahead of time the length of the encoded input, we
	// choose to be conservative in sizing the buffer by sticking with the default
	// (64 bytes) and growing as necessary. This is done to reduce memory spikes.
	err := json.NewEncoder(&buf).Encode(input)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
