// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package libpf

import (
	"encoding"
	"encoding/json"
	"fmt"
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

const (
	// UnknownInterp signifies that the interpreter is unknown.
	UnknownInterp InterpType = 0
	// PHP identifies the PHP interpreter.
	PHP InterpType = 2
	// PHPJIT identifes PHP JIT processes.
	PHPJIT InterpType = 9
	// Python identifies the Python interpreter.
	Python InterpType = 1
	// Native identifies native code.
	Native InterpType = 3
	// Kernel identifies kernel code.
	Kernel InterpType = 4
	// HotSpot identifies the Java HotSpot VM.
	HotSpot InterpType = 5
	// Ruby identifies the Ruby interpreter.
	Ruby InterpType = 6
	// Perl identifies the Perl interpreter.
	Perl InterpType = 7
	// V8 identifies the V8 interpreter.
	V8 InterpType = 8
)

// Frame converts the interpreter type into the corresponding frame type.
func (i InterpType) Frame() FrameType {
	return FrameType(i)
}

var interpTypeToString = map[InterpType]string{
	UnknownInterp: "unknown",
	PHP:           "php",
	PHPJIT:        "phpjit",
	Python:        "python",
	Native:        "native",
	Kernel:        "kernel",
	HotSpot:       "jvm",
	Ruby:          "ruby",
	Perl:          "perl",
	V8:            "v8",
}

// String converts the frame type int to the related string value to be displayed in the UI.
func (i InterpType) String() string {
	if result, ok := interpTypeToString[i]; ok {
		return result
	}
	// nolint:goconst
	return "<invalid>"
}

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

// Convenience shorthands to create various frame types.
//
// Code should not compare against the constants below directly, but instead use the provided
// methods to query the required information (IsError, Interpreter, ...) to improve forward
// compatibility and clarify intentions.
const (
	// UnknownFrame indicates a frame of an unknown interpreter.
	// If this appears, it's likely a bug somewhere.
	UnknownFrame FrameType = 0
	// PHPFrame identifies PHP interpreter frames.
	PHPFrame FrameType = 2
	// PHPJITFrame identifies PHP JIT interpreter frames.
	PHPJITFrame FrameType = 9
	// PythonFrame identifies the Python interpreter frames.
	PythonFrame FrameType = 1
	// NativeFrame identifies native frames.
	NativeFrame FrameType = 3
	// KernelFrame identifies kernel frames.
	KernelFrame FrameType = 4
	// HotSpotFrame identifies Java HotSpot VM frames.
	HotSpotFrame FrameType = 5
	// RubyFrame identifies the Ruby interpreter frames.
	RubyFrame FrameType = 6
	// PerlFrame identifies the Perl interpreter frames.
	PerlFrame FrameType = 7
	// V8Frame identifies the V8 interpreter frames.
	V8Frame FrameType = 8
	// AbortFrame identifies frames that report that further unwinding was aborted due to an error.
	AbortFrame FrameType = 255
)

// Interpreter returns the interpreter that produced the frame.
func (ty FrameType) Interpreter() (ity InterpType, ok bool) {
	switch ty {
	case AbortFrame, UnknownFrame:
		return UnknownInterp, false
	default:
		return InterpType(ty & ^128), true
	}
}

// IsInterpType checks whether the frame type belongs to the given interpreter.
func (ty FrameType) IsInterpType(ity InterpType) bool {
	ity2, ok := ty.Interpreter()
	if !ok {
		return false
	}
	return ity == ity2
}

// Error adds the error bit into the frame type.
func (ty FrameType) Error() FrameType {
	return ty | 128
}

// IsError checks whether the frame is an error frame.
func (ty FrameType) IsError() bool {
	return ty&128 != 0
}

// String implements the Stringer interface.
func (ty FrameType) String() string {
	switch ty {
	case AbortFrame:
		return "abort-marker"
	default:
		interp, _ := ty.Interpreter()
		if ty.IsError() {
			return fmt.Sprintf("%s-error", interp)
		}
		return interp.String()
	}
}

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
