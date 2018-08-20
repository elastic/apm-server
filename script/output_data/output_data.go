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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/beat"
)

func main() {

	err := generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func generate() error {
	var checked = map[string]struct{}{}

	for _, mapping := range beater.V1Routes {

		data, err := loader.LoadData(filepath.Join("..", "testdata", mapping.Processor.Name(), "payload.json"))
		if err != nil {
			return err
		}

		err = mapping.Processor.Validate(data)
		if err != nil {
			return err
		}

		metadata, payload, err := mapping.Processor.Decode(data)
		if err != nil {
			return err
		}

		tctx := &transform.Context{
			Metadata: *metadata,
		}

		var events []beat.Event
		for _, transformable := range payload {
			events = append(events, transformable.Transform(tctx)...)
		}

		for _, d := range events {
			n, err := d.GetValue("processor.name")
			if err != nil {
				return err
			}
			event, err := d.GetValue("processor.event")
			if err != nil {
				return err
			}

			key := n.(string) + "-" + event.(string)

			if _, ok := checked[key]; !ok {
				checked[key] = struct{}{}
				file := filepath.Join("docs/data/elasticsearch", event.(string)+".json")

				output, err := json.MarshalIndent(d.Fields, "", "    ")
				if err != nil {
					return err
				}
				if len(output) > 0 && output[len(output)-1] != '\n' {
					output = append(output, '\n')
				}
				err = ioutil.WriteFile(file, output, 0644)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
