package loader

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"

	"github.com/elastic/apm-server/processor"
)

func LoadData(processorName string, file string) (processor.Intake, error) {
	return inputData(processorName, findFile(file))
}

func LoadValidData(processorName string) (processor.Intake, error) {
	return loadData(processorName, true)
}

func LoadInvalidData(processorName string) (processor.Intake, error) {
	return loadData(processorName, false)
}

func UnmarshalValidData(processorName string) (map[string]interface{}, error) {
	p, _ := buildPath(processorName, true)
	f, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	err = json.Unmarshal(f, &data)
	return data, err
}

func findFile(fileName string) string {
	_, current, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(current), "..", fileName)
}

func loadData(processorName string, valid bool) (processor.Intake, error) {
	p, err := buildPath(processorName, valid)
	if err != nil {
		return processor.Intake{}, err
	}
	return inputData(processorName, p)
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
	return findFile(filepath.Join("data", valid, file)), nil
}

func inputData(processorName string, filePath string) (processor.Intake, error) {
	input := processor.Intake{}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return input, err
	}
	input.Data = data
	if processorName != "sourcemap" {
		return input, nil
	}

	var smapData map[string]string
	err = json.Unmarshal(data, &smapData)
	if err != nil {
		return input, err
	}
	return processor.Intake{
		Data:           []byte(smapData["sourcemap"]),
		ServiceName:    smapData["service_name"],
		ServiceVersion: smapData["service_version"],
		BundleFilepath: smapData["bundle_filepath"],
	}, nil
}
