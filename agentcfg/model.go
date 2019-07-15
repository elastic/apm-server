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

package agentcfg

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
)

// Doc represents an elasticsearch document
type Doc struct {
	Settings Settings
	ID       string
}

// Settings hold agent configuration
type Settings map[string]string

// NewDoc unmarshals given byte slice into t Doc instance
func NewDoc(in []byte) (*Doc, error) {
	if len(in) == 0 {
		return &Doc{}, nil
	}
	var tmp tmpDoc
	if err := json.Unmarshal(in, &tmp); err != nil {
		return nil, err
	}

	h := md5.New()
	var keys []string
	for k := range tmp.Source.Settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string)
	for _, k := range keys {
		out[k] = fmt.Sprintf("%v", tmp.Source.Settings[k])
		h.Write([]byte(fmt.Sprintf("%s_%v", k, tmp.Source.Settings[k])))
	}

	return &Doc{
		ID:       fmt.Sprintf("%x", h.Sum(nil)),
		Settings: out,
	}, nil
}

type tmpDoc struct {
	Source struct {
		Settings map[string]interface{} `json:"settings"`
	} `json:"_source"`
}
