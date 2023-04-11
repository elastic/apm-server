// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package libpf

import (
	"encoding"
	"encoding/json"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/basehash"
)

// UnixTime32 is another type to represent seconds since epoch.
// In most cases 32bit time values are good enough until year 2106.
// Our time series database backend uses this type for TimeStamps as well,
// so there is no need to use a different type than uint32.
// Also, Go's semantics on map[time.Time] are particularly nasty footguns,
// and since the code is mostly dealing with UNIX timestamps, we may
// as well use uint32s instead.
// To restore some semblance of type safety, we declare a type alias here.
type UnixTime32 uint32

func (t *UnixTime32) MarshalJSON() ([]byte, error) {
	return time.Unix(int64(*t), 0).UTC().MarshalJSON()
}

// Compile-time interface checks
var _ json.Marshaler = (*UnixTime32)(nil)

// NowAsUInt32 is a convenience function to avoid code repetition
func NowAsUInt32() uint32 {
	return uint32(time.Now().Unix())
}

// PID represent Unix Process ID (pid_t)
type PID int32

// FileID is used for unique identifiers for files
type FileID struct {
	basehash.Hash128
}

// UnsymbolizedFileID is used as 128-bit FileID when symbolization fails.
var UnsymbolizedFileID = NewFileID(math.MaxUint64, math.MaxUint64)

func NewFileID(hi, lo uint64) FileID {
	return FileID{basehash.New128(hi, lo)}
}

// FileIDFromBytes parses a byte slice into the internal data representation for a file ID.
func FileIDFromBytes(b []byte) (FileID, error) {
	// We need to check for nil since byte slice fields in protobuf messages can be optional.
	// Until improved message validation and deserialization is added, this check will prevent
	// panics.
	if b == nil {
		return FileID{}, nil
	}
	hash, err := basehash.New128FromBytes(b)
	if err != nil {
		return FileID{}, err
	}
	return FileID{hash}, nil
}

// FileIDFromString parses a hexadecimal notation of a file ID into the internal data
// representation.
func FileIDFromString(s string) (FileID, error) {
	hash128, err := basehash.New128FromString(s)
	if err != nil {
		return FileID{}, err
	}
	return FileID{hash128}, nil
}

func (f FileID) Equal(other FileID) bool {
	return f.Hash128.Equal(other.Hash128)
}

func (f FileID) Less(other FileID) bool {
	return f.Hash128.Less(other.Hash128)
}

// Compile-time interface checks
var _ encoding.TextUnmarshaler = (*FileID)(nil)
var _ encoding.TextMarshaler = (*FileID)(nil)

// PackageID is used for unique identifiers for packages
type PackageID struct {
	basehash.Hash128
}

// PackageIDFromBytes parses a byte slice into the internal data representation for a PackageID.
func PackageIDFromBytes(b []byte) (PackageID, error) {
	hash, err := basehash.New128FromBytes(b)
	if err != nil {
		return PackageID{}, err
	}
	return PackageID{hash}, nil
}

// Equal returns true if both PackageIDs are equal.
func (h PackageID) Equal(other PackageID) bool {
	return h.Hash128.Equal(other.Hash128)
}

// String returns the string representation for the package ID.
func (h PackageID) String() string {
	return h.StringNoQuotes()
}

// PackageIDFromString returns a PackageID from its string representation.
func PackageIDFromString(str string) (PackageID, error) {
	hash128, err := basehash.New128FromString(str)
	if err != nil {
		return PackageID{}, err
	}
	return PackageID{hash128}, nil
}

// TraceHash represents the unique hash of a trace
type TraceHash struct {
	basehash.Hash128
}

func NewTraceHash(hi, lo uint64) TraceHash {
	return TraceHash{basehash.New128(hi, lo)}
}

// TraceHashFromBytes parses a byte slice of a trace hash into the internal data representation.
func TraceHashFromBytes(b []byte) (TraceHash, error) {
	hash, err := basehash.New128FromBytes(b)
	if err != nil {
		return TraceHash{}, err
	}
	return TraceHash{hash}, nil
}

// TraceHashFromString parses a hexadecimal notation of a trace hash into the internal data
// representation.
func TraceHashFromString(s string) (TraceHash, error) {
	hash128, err := basehash.New128FromString(s)
	if err != nil {
		return TraceHash{}, err
	}
	return TraceHash{hash128}, nil
}

func (h TraceHash) Equal(other TraceHash) bool {
	return h.Hash128.Equal(other.Hash128)
}

func (h TraceHash) Less(other TraceHash) bool {
	return h.Hash128.Less(other.Hash128)
}

// Compile-time interface checks
var _ encoding.TextUnmarshaler = (*TraceHash)(nil)
var _ encoding.TextMarshaler = (*TraceHash)(nil)

// HostOrPodID represents the unique identifier for a host or a Pod.
type HostOrPodID basehash.Hash64

func (h HostOrPodID) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatUint(uint64(h), 16))), nil
}

// MarshalText implements the encoding.TextMarshaler interface, so we can
// marshal (from JSON) a map using a HostOrPodID as a key
func (h HostOrPodID) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatUint(uint64(h), 16)), nil
}

// Compile-time interface checks
var _ encoding.TextMarshaler = (*HostOrPodID)(nil)
var _ json.Marshaler = (*HostOrPodID)(nil)

// AddressOrLineno represents a line number in an interpreted file or an offset into
// a native file. TODO(thomasdullien): check with regards to JSON marshaling/demarshaling.
type AddressOrLineno uint64

// Address represents an address, or offset within a process
type Address uint64

// InterpVersion represents the version of an interpreter
type InterpVersion string

// SourceLineno represents a line number within a source file. It is intended to be used for the
// source line numbers associated with offsets in native code, or for source line numbers in
// interpreted code.
type SourceLineno uint64

// InterpType variables can hold one of the interpreter type values defined below.
// TODO(thomasdullien): Refactor the name to "FrameType"?
type InterpType int

// FrameType defines the type of frame. This usually corresponds to the interpreter type that
// emitted it, but can additionally contain meta-information like error frames.
//
// A frame type can represent one of the following things:
//
//   - A successfully unwound frame. This is represented simply as the `InterpType` ID.
//   - A partial (non-critical failure), indicated by ORing the `InterpType` ID with the error bit.
//   - A fatal failure that caused further unwinding to be aborted. This is indicated using the
//     special value support.FrameMarkerAbort (0xFF). It thus also contains the error bit, but
//     does not fit into the `InterpType` enum.
type FrameType int

// SourceType identifies the different forms of source code files that we may deal with.
type SourceType int

// Trace represents a stack trace. Each tuple (Files[i], Linenos[i]) represents a
// stack frame via the file ID and line number at the offset i in the trace. The
// information for the most recently called function is at offset 0.
type Trace struct {
	Files         []FileID
	Linenos       []AddressOrLineno
	FrameTypes    []FrameType
	Comm          string
	PodName       string
	ContainerName string
	Hash          TraceHash
	PID           PID
}

type TraceAndMetadata struct {
	Hash          TraceHash
	Comm          string
	PodName       string
	ContainerName string
}

type TraceAndCounts struct {
	Hash          TraceHash
	Timestamp     UnixTime32
	Count         uint16
	Comm          string
	PodName       string
	ContainerName string
}

type FrameMetadata struct {
	FileID         FileID
	SourceID       FileID
	AddressOrLine  AddressOrLineno
	LineNumber     SourceLineno
	SourceType     SourceType
	FunctionOffset uint32
	FunctionName   string
	Filename       string
}

// Void allows to use maps as sets without memory allocation for the values.
// From the "Go Programming Language":
//
//	The struct type with no fields is called the empty struct, written struct{}. It has size zero
//	and carries no information but may be useful nonetheless. Some Go programmers
//	use it instead of bool as the value type of a map that represents a set, to emphasize
//	that only the keys are significant, but the space saving is marginal and the syntax more
//	cumbersome, so we generally avoid it.
type Void struct{}

// AddJitter adds +/- jitter (jitter is [0..1]) to baseDuration
func AddJitter(baseDuration time.Duration, jitter float64) time.Duration {
	if jitter < 0.0 || jitter > 1.0 {
		return baseDuration
	}
	// nolint:gosec
	return time.Duration((1 + jitter - 2*jitter*rand.Float64()) * float64(baseDuration))
}
