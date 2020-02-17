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

package loader

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/elastic/apm-server/decoder"
)

func LoadJSON(fileName string) (map[string]interface{}, error) {
	return unmarshal(fileReader(fileName))
}

func LoadNDJSON(fileName string) ([]byte, error) {
	return read(NDJSONFileReader(fileName))
}

func LoadData(fileName string) ([]byte, error) {
	return read(fileReader(fileName))
}

func FindFile(fileInfo ...string) (string, error) {
	_, current, _, _ := runtime.Caller(0)
	f := []string{filepath.Dir(current), ".."}
	f = append(f, fileInfo...)
	p := filepath.Join(f...)
	_, err := os.Stat(p)
	return p, err
}

func fileReader(filePath string) (io.ReadCloser, error) {
	f, err := FindFile(filePath)
	if err != nil {
		return nil, err
	}
	return os.Open(f)
}

func NDJSONFileReader(filePath string) (io.ReadCloser, error) {
	reader, err := fileReader(filePath)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(reader)
	scanner.Split(splitEvents)
	var buf bytes.Buffer
	for scanner.Scan() {
		buf.Write(scanner.Bytes())
		// ndjson separator
		buf.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// splitEvents makes an Scanner to return an NDJSON event on each read call
func splitEvents(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	var buf bytes.Buffer
	var start, end int
	for {
		if end = bytes.IndexByte(data[start:], '}') + start + 1; end >= start+1 {
			buf.Write(data[start:end])
			opened := occurrences('{', buf.Bytes())
			closed := occurrences('}', buf.Bytes())
			start = end
			// if the number of open braces is the same as the closed, we have an entire event
			if opened == closed {
				var b = buf.Bytes()
				for _, x := range [][]byte{{'\r'}, {'\n'}, {'\t'}} {
					b = bytes.Replace(b, x, nil, -1)
				}
				// return number of positions to advance the scanner, and the event
				return start, b, nil
			}
		} else {
			break
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	// ask for more data to the scanner, as we didn't reach the end of the event yet
	return 0, nil, nil
}

func occurrences(b byte, bs []byte) int {
	var count int
	for i := 0; i < len(bs); i++ {
		if bs[i] == b {
			count++
		}
	}
	return count
}

func read(r io.ReadCloser, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

func unmarshal(r io.ReadCloser, err error) (map[string]interface{}, error) {
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return decoder.DecodeJSONData(r)
}
