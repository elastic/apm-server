// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package common

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/libpf"
)

func base64URLEncode(in []byte) string {
	return base64.RawURLEncoding.EncodeToString(in)
}

// EncodeStackTraceID encodes a StackTraceID into its ES representation.
func EncodeStackTraceID(hash libpf.TraceHash) string {
	return base64URLEncode(hash.Bytes())
}

// EncodeFileID encodes a FileID into its ES representation.
func EncodeFileID(fileID libpf.FileID) string {
	return base64URLEncode(fileID.Bytes())
}

type FrameID [24]byte

func (f FrameID) Bytes() []byte {
	return f[0:24]
}

func (f FrameID) FileIDBytes() []byte {
	return f[0:16]
}

// MakeFrameID creates a single FrameID by concatenating a fileID and
// an addressOrLineno value.
func MakeFrameID(fileID libpf.FileID, addressOrLineno uint64) FrameID {
	var frameID [24]byte

	// DocID is the base64-encoded FileID+address.
	binary.BigEndian.PutUint64(frameID[:8], fileID.Hi())
	binary.BigEndian.PutUint64(frameID[8:16], fileID.Lo())
	binary.BigEndian.PutUint64(frameID[16:], addressOrLineno)
	return frameID
}

// EncodeFrameID creates a single frameID by concatenating a fileID and
// an addressOrLineno value. The result is returned as base64url encoded string.
func EncodeFrameID(fileID libpf.FileID, addressOrLineno uint64) string {
	return base64URLEncode(MakeFrameID(fileID, addressOrLineno).Bytes())
}

// EncodeFrameIDs reverses the order of frames to allow for prefix compression (leaf frame last)
// and encodes the frameIDs as a single concatenated base64url string.
// Prefix compression, as well as the future synthetic source feature, requires a
// single value instead of an array of values.
func EncodeFrameIDs(fileIDs []libpf.FileID, addressOrLinenos []libpf.AddressOrLineno) string {
	var buf bytes.Buffer
	for i := len(addressOrLinenos) - 1; i >= 0; i-- {
		buf.Write(fileIDs[i].Bytes())
		_ = binary.Write(io.Writer(&buf), binary.BigEndian, addressOrLinenos[i])
	}
	return base64URLEncode(buf.Bytes())
}

// EncodeFrameTypes applies run-length encoding to the frame types in reverse order
// and returns the results as base64url encoded string.
func EncodeFrameTypes(frameTypes []libpf.FrameType) string {
	var buf bytes.Buffer
	RunLengthEncodeReverse(frameTypes, &buf,
		func(frameType libpf.FrameType) []byte {
			return []byte{byte(frameType)}
		})
	return base64URLEncode(buf.Bytes())
}
