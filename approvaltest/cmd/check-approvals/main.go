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

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"

	"github.com/elastic/apm-server/approvaltest"
)

func main() {
	os.Exit(approval())
}

func approval() int {
	cwd, _ := os.Getwd()
	receivedFiles := findFiles(cwd, approvaltest.ReceivedSuffix)

	for _, rf := range receivedFiles {
		af := strings.TrimSuffix(rf, approvaltest.ReceivedSuffix) + approvaltest.ApprovedSuffix

		var approved, received interface{}
		if err := decodeJSONFile(rf, &received); err != nil {
			fmt.Println("Could not create diff ", err)
			return 3
		}
		if err := decodeJSONFile(af, &approved); err != nil && !os.IsNotExist(err) {
			fmt.Println("Could not create diff ", err)
			return 3
		}

		diff := cmp.Diff(approved, received)
		added := color.New(color.FgBlack, color.BgGreen).SprintFunc()
		deleted := color.New(color.FgBlack, color.BgRed).SprintFunc()
		scanner := bufio.NewScanner(strings.NewReader(diff))
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) > 0 {
				switch line[0] {
				case '-':
					line = deleted(line)
				case '+':
					line = added(line)
				}
			}
			fmt.Println(line)
		}

		fmt.Println(rf)
		fmt.Println("\nApprove Changes? (y/n)")
		reader := bufio.NewReader(os.Stdin)
		input, _, _ := reader.ReadRune()
		switch input {
		case 'y':
			approvedPath := strings.Replace(rf, approvaltest.ReceivedSuffix, approvaltest.ApprovedSuffix, 1)
			os.Rename(rf, approvedPath)
		}
	}
	return 0
}

func findFiles(rootDir string, suffix string) []string {
	files := []string{}
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func decodeJSONFile(path string, out interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return fmt.Errorf("cannot unmarshal file %q: %w", path, err)
	}
	return nil
}
