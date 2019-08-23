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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yudai/gojsondiff"

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

	verifyErr := ApproveJSON(resultmap, name, map[string]string{})
	if verifyErr != nil {
		assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", name, verifyErr.Error()))
	}
}

// ApproveEvents iterates over given events and ensures per event that data are approved.
func ApproveEvents(events []beat.Event, name string, ignored map[string]string) error {
	// extract Fields and write to received.json
	eventFields := make([]common.MapStr, len(events))
	for idx, event := range events {
		eventFields[idx] = event.Fields
		eventFields[idx]["@timestamp"] = event.Timestamp
	}

	receivedJSON := map[string]interface{}{"events": eventFields}
	return ApproveJSON(receivedJSON, name, ignored)
}

// ApproveJSON iterates over received data and verifies them against already approved data. If data differ a message
// will be print suggesting how to proceed with approval procedure.
func ApproveJSON(received map[string]interface{}, name string, ignored map[string]string) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, name)
	receivedPath := path + ReceivedSuffix

	r, _ := json.MarshalIndent(received, "", "    ")
	ioutil.WriteFile(receivedPath, r, 0644)
	received, _, diff, err := Compare(path, ignored)
	if err != nil {
		return err
	}
	if len(diff.Deltas()) > 0 {
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

func ignoredKey(data *map[string]interface{}, ignored map[string]string) {
	for k, v := range *data {
		if ignoreVal, ok := ignored[k]; ok {
			(*data)[k] = ignoreVal
		} else if vm, ok := v.(map[string]interface{}); ok {
			ignoredKey(&vm, ignored)
		} else if vm, ok := v.([]interface{}); ok {
			for _, e := range vm {
				if em, ok := e.(map[string]interface{}); ok {
					ignoredKey(&em, ignored)

				}
			}
		}
	}
}

// Compare compares given data to approved data and returns diff if not equal.
func Compare(path string, ignored map[string]string) (map[string]interface{}, []byte, gojsondiff.Diff, error) {
	rec, err := ioutil.ReadFile(path + ReceivedSuffix)
	if err != nil {
		fmt.Println("Cannot read file ", path, err)
		return nil, nil, nil, err
	}
	var data map[string]interface{}
	err = json.Unmarshal(rec, &data)
	if err != nil {
		fmt.Println("Cannot unmarshal received file ", path, err)
		return nil, nil, nil, err
	}
	ignoredKey(&data, ignored)
	received, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Cannot marshal received data", err)
		return nil, nil, nil, err
	}

	approved, err := ioutil.ReadFile(path + ApprovedSuffix)
	if err != nil {
		approved = []byte("{}")
	}

	differ := gojsondiff.New()
	d, err := differ.Compare(approved, received)
	return data, approved, d, err
}
