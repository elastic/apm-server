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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

// ApprovedSuffix signals a file has been reviewed and approved
const ApprovedSuffix = ".approved.json"

// ReceivedSuffix signals a file has changed and not yet been approved
const ReceivedSuffix = ".received.json"

// AssertApproveResult tests that given result equals an already approved result, and fails otherwise.
func AssertApproveResult(t *testing.T, name string, actualResult []byte) {
	var resultmap map[string]interface{}
	err := json.Unmarshal(actualResult, &resultmap)
	require.NoError(t, err)

	verifyErr := ApproveJSON(resultmap, name)
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

// ApproveJSON iterates over received data and verifies them against already approved data. If data differ a message
// will be print suggesting how to proceed with approval procedure.
func ApproveJSON(received map[string]interface{}, name string, ignored ...string) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, name)
	receivedPath := path + ReceivedSuffix

	r, _ := json.MarshalIndent(received, "", "    ")
	ioutil.WriteFile(receivedPath, r, 0644)
	received, _, diff, err := Compare(path, ignored...)
	if err != nil {
		return err
	}
	if diff != "" {
		r, _ := json.MarshalIndent(received, "", "    ")
		if len(r) > 0 && r[len(r)-1] != '\n' {
			r = append(r, '\n')
		}
		ioutil.WriteFile(receivedPath, r, 0644)
		return errors.New("received data differs from approved data. Run 'make update' and then 'approvals' to verify the diff")
	}

	os.Remove(receivedPath)
	return nil
}

// Compare compares given data to approved data and returns diff if not equal.
func Compare(path string, ignoredFields ...string) (map[string]interface{}, map[string]interface{}, string, error) {
	receivedf, err := os.Open(path + ReceivedSuffix)
	if err != nil {
		fmt.Println("Cannot read file ", path, err)
		return nil, nil, "", err
	}
	defer receivedf.Close()

	var received map[string]interface{}
	if err := json.NewDecoder(receivedf).Decode(&received); err != nil {
		fmt.Println("Cannot unmarshal received file ", path, err)
		return nil, nil, "", err
	}

	var approved map[string]interface{}
	approvedf, err := os.Open(path + ApprovedSuffix)
	if err == nil {
		defer approvedf.Close()
		if err := json.NewDecoder(approvedf).Decode(&approved); err != nil {
			fmt.Println("Cannot unmarshal approved file ", path, err)
			return nil, nil, "", err
		}
	}

	ignored := make(map[string]bool)
	for _, field := range ignoredFields {
		ignored[field] = true
	}
	opts := []cmp.Option{
		cmp.FilterPath(func(p cmp.Path) bool {
			if mi, ok := p.Last().(cmp.MapIndex); ok {
				return ignored[mi.Key().String()]
			}
			return false
		}, cmp.Ignore()),
	}

	diff := cmp.Diff(approved, received, opts...)
	return received, approved, diff, nil
}
