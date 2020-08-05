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

package approvals

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

// ApprovedSuffix signals a file has been reviewed and approved
const ApprovedSuffix = ".approved.json"

// ReceivedSuffix signals a file has changed and not yet been approved
const ReceivedSuffix = ".received.json"

// AssertApproveResult tests that given result equals an already approved result, and fails otherwise.
func AssertApproveResult(t *testing.T, name string, actualResult []byte, ignored ...string) {
	var resultmap map[string]interface{}
	err := json.Unmarshal(actualResult, &resultmap)
	require.NoError(t, err)

	verifyErr := ApproveJSON(resultmap, name, ignored...)
	if verifyErr != nil {
		assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", name, verifyErr.Error()))
	}
}

// ApproveEvents iterates over given events and ensures per event that data are approved.
func ApproveEvents(events []beat.Event, name string, ignored ...string) error {
	// extract Fields and write to received.json
	eventFields := make([]common.MapStr, len(events))
	for idx, event := range events {
		eventFields[idx] = event.Fields
		eventFields[idx]["@timestamp"] = event.Timestamp
	}

	receivedJSON := map[string]interface{}{"events": eventFields}
	return ApproveJSON(receivedJSON, name, ignored...)
}

// ApproveJSON iterates over received data and verifies them against already approved data.
// If data differ a message will be print suggesting how to proceed with approval procedure.
func ApproveJSON(received map[string]interface{}, name string, ignored ...string) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, name)
	approved := readApproved(path)
	if approved == nil {
		approved = map[string]interface{}(nil)
	}

	// Encode/decode the received data to convert common.MapStr to map[string]interface{}, etc.
	data, _ := json.Marshal(received)
	json.Unmarshal(data, &received)

	if diff := CompareObjects(received, approved, ignored...); diff != "" {
		f, err := os.OpenFile(path+ReceivedSuffix, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "    ")
		if err := enc.Encode(received); err != nil {
			return err
		}
		return errors.New("received data differs from approved data. Run 'make update check-approvals' to verify the diff")
	}
	return nil
}

// Compare compares given data to approved data and returns diff if not equal.
func Compare(path string, ignoredFields ...string) (received interface{}, approved interface{}, diff string, _ error) {
	receivedf, err := os.Open(path + ReceivedSuffix)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cannot read file %q: %w", path, err)
	}
	defer receivedf.Close()

	if err := json.NewDecoder(receivedf).Decode(&received); err != nil {
		return nil, nil, "", fmt.Errorf("cannot unmarshal file %q: %w", path, err)
	}

	approved = readApproved(path)
	return received, approved, CompareObjects(received, approved, ignoredFields...), nil
}

func readApproved(path string) interface{} {
	approvedf, err := os.Open(path + ApprovedSuffix)
	if err != nil {
		return nil
	}
	defer approvedf.Close()

	var approved interface{}
	if err := json.NewDecoder(approvedf).Decode(&approved); err != nil {
		panic(fmt.Errorf("cannot unmarshal file %q: %w", path, err))
	}
	return approved
}

// CompareObjects compares two given objects, returning a
// diff if not equal, and an empty string if they are equal.
func CompareObjects(received, approved interface{}, ignoredFields ...string) (diff string) {
	ignored := make([][]string, len(ignoredFields))
	for i, field := range ignoredFields {
		ignored[i] = strings.Split(field, ".")
	}
	sort.Slice(ignored, func(i, j int) bool {
		return len(ignored[i]) < len(ignored[j])
	})

	opts := []cmp.Option{
		cmp.FilterPath(func(p cmp.Path) bool {
			for _, ignored := range ignored {
				if len(ignored) > len(p) {
					continue
				}
				for i := 0; i < len(ignored); i++ {
					mi, ok := p.Index(-i - 1).(cmp.MapIndex)
					if !ok {
						break
					}
					if mi.Key().String() != ignored[len(ignored)-1-i] {
						break
					}
					if i == 0 {
						return true
					}
				}
			}
			return false
		}, cmp.Ignore()),
	}
	return cmp.Diff(approved, received, opts...)
}
