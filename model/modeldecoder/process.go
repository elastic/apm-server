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

package modeldecoder

import (
	"errors"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/utility"
)

func decodeProcess(input interface{}, err error) (*metadata.Process, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for process")
	}
	decoder := utility.ManualDecoder{}
	process := metadata.Process{
		Ppid:  decoder.IntPtr(raw, "ppid"),
		Title: decoder.StringPtr(raw, "title"),
		Argv:  decoder.StringArr(raw, "argv"),
	}
	if pid := decoder.IntPtr(raw, "pid"); pid != nil {
		process.Pid = *pid
	}
	return &process, decoder.Err
}
