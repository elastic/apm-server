package tests

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yudai/gojsondiff"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

const ApprovedSuffix = ".approved.json"
const ReceivedSuffix = ".received.json"

func ApproveJson(received map[string]interface{}, name string, ignored []string) error {
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
		ioutil.WriteFile(receivedPath, r, 0644)
		return errors.New(fmt.Sprintf("Received data differ from approved data. Run 'make update' and then 'approvals' to verify the Diff."))
	}

	os.Remove(receivedPath)
	return nil
}

func hasKey(coll []string, k string) bool {
	for _, entry := range coll {
		if entry == k {
			return true
		}
	}
	return false
}

func ignoredKey(data *map[string]interface{}, ignoredKeys []string) {
	for k, v := range *data {
		if hasKey(ignoredKeys, k) {
			(*data)[k] = "Ignored"
		} else if vm, ok := v.(map[string]interface{}); ok {
			ignoredKey(&vm, ignoredKeys)
		} else if vm, ok := v.([]interface{}); ok {
			for _, e := range vm {
				if em, ok := e.(map[string]interface{}); ok {
					ignoredKey(&em, ignoredKeys)

				}
			}
		}
	}
}

func Compare(path string, ignored []string) (map[string]interface{}, []byte, gojsondiff.Diff, error) {
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

type RequestInfo struct {
	Name string
	Path string
}

func TestProcessRequests(t *testing.T, p processor.Processor, requestInfo []RequestInfo, ignoredKeys []string) {
	assert := assert.New(t)
	for _, info := range requestInfo {
		data, err := LoadData(info.Path)
		assert.Nil(err)

		err = p.Validate(data)
		assert.NoError(err)

		events, err := p.Transform(data)
		assert.NoError(err)

		// extract Fields and write to received.json
		eventFields := make([]common.MapStr, len(events))
		for idx, event := range events {
			eventFields[idx] = event.Fields
			eventFields[idx]["@timestamp"] = event.Timestamp
		}

		receivedJson := map[string]interface{}{"events": eventFields}
		verifyErr := ApproveJson(receivedJson, info.Name, ignoredKeys)
		if verifyErr != nil {
			assert.Fail(fmt.Sprintf("Test %s failed with error: %s", info.Name, verifyErr.Error()))

		}
	}
}
