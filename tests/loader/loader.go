package loader

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/elastic/apm-server/decoder"
)

func findFile(fileName string) (string, error) {
	_, current, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(current), "..", fileName), nil
}

func fileReader(filePath string, err error) (io.ReadCloser, error) {
	if err != nil {
		return nil, err
	}
	return os.Open(filePath)
}

func readFile(filePath string, err error) ([]byte, error) {
	var f io.Reader
	f, err = fileReader(filePath, err)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(f)
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
	var r io.ReadCloser
	r, err = fileReader(filePath, err)
	if err != nil {
		return data, err
	}
	return decoder.DecodeJSONData(r)
}
