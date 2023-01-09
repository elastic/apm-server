// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package common

import "io"

// toByteArrayFn is the function type for runLengthEncodeReverse() callback
// to convert an item into an array of bytes.
type toByteArrayFn[T comparable] func(value T) []byte

// RunLengthEncodeReverse applies a run-length encoding to the reversed input array.
// The output is a binary stream of 1-byte length and the binary representation of the
// object.
// E.g. an uint8 array like ['a', 'a', 'x', 'x', 'x', 'x', 'x'] is converted into
// the byte array [5, 'x', 2, 'a'].
func RunLengthEncodeReverse[T comparable](
	values []T, writer io.Writer, toBytes toByteArrayFn[T]) {
	if len(values) == 0 {
		return
	}

	l := 1
	cur := values[len(values)-1]

	write := func() {
		_, _ = writer.Write([]byte{byte(l)})
		_, _ = writer.Write(toBytes(cur))
	}

	for i := len(values) - 2; i >= 0; i-- {
		next := values[i]

		if next == cur && l < 255 {
			l++
			continue
		}

		write()
		l = 1
		cur = next
	}

	write()
}
