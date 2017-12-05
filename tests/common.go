package tests

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

func UnmarshalData(file string, data interface{}) error {
	_, current, _, _ := runtime.Caller(0)
	return unmarshalData(filepath.Join(filepath.Dir(current), "..", file), data)
}

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

func UnmarshalValidData(dataType string, data interface{}) error {
	path, err := buildPath(dataType, true)
	if err != nil {
		return err
	}
	return unmarshalData(path, data)
}

func UnmarshalInvalidData(dataType string, data interface{}) error {
	path, err := buildPath(dataType, false)
	if err != nil {
		return err
	}
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
			file = "error_payload/no_service.json"
		}
	case "transaction":
		if validData {
			file = "transaction/payload.json"
		} else {
			file = "transaction_payload/no_service.json"
		}
	case "sourcemap":
		if validData {
			file = "sourcemap/payload.json"
		} else {
			file = "sourcemap/no_service_version.json"
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
