// Package arg2 contains tchannel thrift Arg2 interfaces for external use.
//
// These interfaces are currently unstable, and aren't covered by the API
// backwards-compatibility guarantee.
package arg2

import (
	"encoding/binary"
	"fmt"
	"io"
)

// KeyValIterator is a iterator for reading tchannel-thrift Arg2 Scheme,
// which has key/value pairs (k~2 v~2).
// NOTE: to be optimized for performance, we try to limit the allocation
// done in the process of iteration.
type KeyValIterator struct {
	arg2Payload           []byte
	leftPairCount         int
	keyOffset             int
	valueOffset, valueLen int
}

// NewKeyValIterator inits a KeyValIterator with the buffer pointing at
// start of Arg2. Return io.EOF if no iterator is available.
// NOTE: tchannel-thrift Arg Scheme starts with number of key/value pair.
func NewKeyValIterator(arg2Payload []byte) (KeyValIterator, error) {
	if len(arg2Payload) < 2 {
		return KeyValIterator{}, io.EOF
	}

	return KeyValIterator{
		valueOffset:   2, // nh has 2B offset
		leftPairCount: int(binary.BigEndian.Uint16(arg2Payload[0:2])),
		arg2Payload:   arg2Payload,
	}.Next()
}

// Key Returns the key.
func (i KeyValIterator) Key() []byte {
	return i.arg2Payload[i.keyOffset : i.valueOffset-2 /*2B length*/]
}

// Value returns value.
func (i KeyValIterator) Value() []byte {
	return i.arg2Payload[i.valueOffset : i.valueOffset+i.valueLen]
}

// Next returns next iterator. Return io.EOF if no more key/value pair is
// available.
func (i KeyValIterator) Next() (KeyValIterator, error) {
	if i.leftPairCount <= 0 {
		return KeyValIterator{}, io.EOF
	}

	arg2Len := len(i.arg2Payload)
	cur := i.valueOffset + i.valueLen
	if cur+2 > arg2Len {
		return KeyValIterator{}, fmt.Errorf("invalid key offset %v (arg2 len %v)", cur, arg2Len)
	}
	keyLen := int(binary.BigEndian.Uint16(i.arg2Payload[cur : cur+2]))
	cur += 2
	keyOffset := cur
	cur += keyLen

	if cur+2 > arg2Len {
		return KeyValIterator{}, fmt.Errorf("invalid value offset %v (key offset %v, key len %v, arg2 len %v)", cur, keyOffset, keyLen, arg2Len)
	}
	valueLen := int(binary.BigEndian.Uint16(i.arg2Payload[cur : cur+2]))
	cur += 2
	valueOffset := cur

	if valueOffset+valueLen > arg2Len {
		return KeyValIterator{}, fmt.Errorf("value exceeds arg2 range (offset %v, len %v, arg2 len %v)", valueOffset, valueLen, arg2Len)
	}

	return KeyValIterator{
		arg2Payload:   i.arg2Payload,
		leftPairCount: i.leftPairCount - 1,
		keyOffset:     keyOffset,
		valueOffset:   valueOffset,
		valueLen:      valueLen,
	}, nil
}
