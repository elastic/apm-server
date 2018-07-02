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
	p := filepath.Join(filepath.Dir(current), "..", fileName)
	_, err := os.Stat(p)
	return p, err
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
	return readFile(buildPath(processorName))
}

func LoadValidData(processorName string) (map[string]interface{}, error) {
	return unmarshalData(buildPath(processorName))
}

func buildPath(processorName string) (string, error) {
	switch processorName {
	case "error", "transaction", "sourcemap":
		return findFile(filepath.Join("..", "testdata", processorName, "payload.json"))
	default:
		return "", errors.New("unknown data type")
	}
}

func unmarshalData(filePath string, err error) (map[string]interface{}, error) {
	var r io.ReadCloser
	r, err = fileReader(filePath, err)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return decoder.DecodeJSONData(r)
}
