// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package common

import (
	"io"
	"testing"
)

type TestTypeA struct {
	S string `json:"str,omitempty"`
	I int    `json:"int,omitempty"`
}

func TestEncodeBody(t *testing.T) {
	tests := map[string]struct {
		input    any
		expected string
	}{
		"normal type": {
			input:    TestTypeA{S: "abc", I: 5},
			expected: `{"str":"abc","int":5}`,
		},
		"pointer type": {
			input:    &TestTypeA{S: "abc", I: 5},
			expected: `{"str":"abc","int":5}`,
		},
	}

	for name, testcase := range tests {
		name := name
		testcase := testcase

		t.Run(name, func(t *testing.T) {
			reader, err := EncodeBody(testcase.input)
			if err != nil {
				t.Fatalf("Failed to JSON encode: %v", err)
			}

			result, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Failed to read body: %v", err)
			}
			result = result[:len(result)-1] // remove trailing \n

			if string(result) != testcase.expected {
				t.Fatalf("Unexpected return. Expected %v, got %v",
					testcase.expected, string(result))
			}
		})
	}
}
