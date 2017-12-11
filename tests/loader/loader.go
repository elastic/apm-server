package loader

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

func findFile(fileName string) (string, error) {
	_, current, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(current), "..", fileName), nil
}

func readFile(filePath string, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(filePath)
}

func LoadData(file string) (map[string]interface{}, error) {
	return unmarshalData(findFile(file))
}

func LoadDataAsBytes(fileName string) ([]byte, error) {
	return readFile(findFile(fileName))
}

func LoadValidDataAsBytes(processorName string) ([]byte, error) {
	return readFile(buildPath(processorName, true))
}

func LoadValidData(processorName string) (map[string]interface{}, error) {
	return unmarshalData(buildPath(processorName, true))
}

func LoadInvalidData(processorName string) (map[string]interface{}, error) {
	return unmarshalData(buildPath(processorName, false))
}

func buildPath(processorName string, validData bool) (string, error) {
	valid := "valid"
	if !validData {
		valid = "invalid"
	}

	var file string
	switch processorName {
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
			file = "sourcemap/no_bundle_filepath.json"
		}
	default:
		return "", errors.New("data type not specified")
	}
	return findFile(filepath.Join("data", valid, file))
}

func unmarshalData(filePath string, err error) (map[string]interface{}, error) {
	var data map[string]interface{}
	input, err := readFile(filePath, err)
	if err == nil {
		err = json.Unmarshal(input, &data)
	}
	return data, err
}
