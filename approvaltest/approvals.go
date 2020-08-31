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

package approvaltest

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	// ApprovedSuffix signals a file has been reviewed and approved.
	ApprovedSuffix = ".approved.json"

	// ReceivedSuffix signals a file has changed and not yet been approved.
	ReceivedSuffix = ".received.json"
)

// ApproveEventDocs compares the given event documents with
// the contents of the file in "<name>.approved.json".
//
// Any specified dynamic fields (e.g. @timestamp, observer.id)
// will be replaced with a static string for comparison.
//
// If the events differ, then the test will fail.
func ApproveEventDocs(t testing.TB, name string, eventDocs [][]byte, dynamic ...string) {
	t.Helper()

	// Rewrite all dynamic fields to have a known value,
	// so dynamic fields don't affect diffs.
	events := make([]interface{}, len(eventDocs))
	for i, doc := range eventDocs {
		for _, field := range dynamic {
			existing := gjson.GetBytes(doc, field)
			if !existing.Exists() {
				continue
			}

			var err error
			doc, err = sjson.SetBytes(doc, field, "dynamic")
			if err != nil {
				t.Fatal(err)
			}
		}

		var event map[string]interface{}
		if err := json.Unmarshal(doc, &event); err != nil {
			t.Fatal(err)
		}
		events[i] = event
	}

	received := map[string]interface{}{"events": events}
	approve(t, name, received)
}

// ApproveJSON compares the given JSON-encoded value with the
// contents of the file "<name>.approved.json", by decoding the
// value and passing it to Approve.
//
// If the value differs, then the test will fail.
func ApproveJSON(t testing.TB, name string, encoded []byte) {
	t.Helper()

	var decoded interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Error(err)
	}
	approve(t, name, decoded)
}

// approve compares the given value with the contents of the file
// "<name>.approved.json".
//
// If the value differs, then the test will fail.
func approve(t testing.TB, name string, received interface{}) {
	t.Helper()

	var approved interface{}
	readApproved(name, &approved)
	if diff := cmp.Diff(approved, received); diff != "" {
		writeReceived(name, received)
		t.Fatalf("%s\n%s\n\n", diff,
			"Run `make update check-approvals` to verify the diff",
		)
	} else {
		// Remove an old *.received.json file if it exists.
		removeReceived(name)
	}
}

func readApproved(name string, approved interface{}) {
	path := name + ApprovedSuffix
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&approved); err != nil {
		panic(fmt.Errorf("cannot unmarshal file %q: %w", path, err))
	}
}

func removeReceived(name string) {
	os.Remove(name + ReceivedSuffix)
}

func writeReceived(name string, received interface{}) {
	f, err := os.Create(name + ReceivedSuffix)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err := enc.Encode(received); err != nil {
		panic(err)
	}
}
