package tests

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

func LoadData(file string) ([]byte, error) {
	_, current, _, _ := runtime.Caller(0)
	return ioutil.ReadFile(filepath.Join(filepath.Dir(current), "..", file))
}

func LoadValidData(dataType string) ([]byte, error) {
	path, err := buildPath(dataType, true)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

func LoadInvalidData(dataType string) ([]byte, error) {
	path, err := buildPath(dataType, false)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

func UnmarshalValidData(json string, data interface{}) error {
	_, current, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(current)
	path := filepath.Join(curDir, "data/valid", json)
	return unmarshalData(path, data)
}

func buildPath(dataType string, validData bool) (string, error) {
	valid := "valid"
	if !validData {
		valid = "invalid"
	}

	var file string
	switch dataType {
	case "error":
		if validData {
			file = "error/payload.json"
		} else {
			file = "error_payload/no_app.json"
		}
	case "transaction":
		if validData {
			file = "transaction/payload.json"
		} else {
			file = "transaction_payload/no_app.json"
		}
	default:
		return "", errors.New("data type not specified.")
	}
	_, current, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(current)
	return filepath.Join(curDir, "data", valid, file), nil
}

func unmarshalData(file string, data interface{}) error {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(input, &data)
}

func StrConcat(pre string, post string, delimiter string) string {
	if pre == "" {
		return post
	}
	return pre + delimiter + post
}

func Contains(seq []string, match string) bool {
	if seq == nil {
		return false
	}
	for _, item := range seq {
		if match == item {
			return true
		}
	}
	return false
}
