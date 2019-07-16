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
	"hash"
	"sort"
	"strings"
)

// Doc represents an elasticsearch document
type Doc struct {
	Settings Settings
	ID       string
}

// Settings hold agent configuration
type Settings map[string]string

// NewDoc unmarshals given byte slice into a Doc instance
func NewDoc(inp []byte) (*Doc, error) {
	var doc Doc
	settings, err := unmarshal(inp)
	if settings == nil || err != nil {
		return &doc, err
	}

	h := md5.New()
	var out = map[string]string{}
	if err := parse(settings, out, "", h); err != nil {
		return &doc, err
	}

	doc.ID = fmt.Sprintf("%x", h.Sum(nil))
	doc.Settings = out
	return &doc, nil
}

func unmarshal(inp []byte) (map[string]interface{}, error) {
	if len(inp) == 0 {
		return nil, nil
	}
	type tmpDoc struct {
		Source struct {
			Settings map[string]interface{} `json:"settings"`
		} `json:"_source"`
	}
	var tmp tmpDoc
	if err := json.Unmarshal(inp, &tmp); err != nil {
		return nil, err
	}
	return tmp.Source.Settings, nil
}

func parse(inp map[string]interface{}, out map[string]string, rootKey string, h hash.Hash) error {
	var keys []string
	for k := range inp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var localkey string
	for _, k := range keys {
		if rootKey == "" {
			localkey = k
		} else {
			localkey = fmt.Sprintf("%s.%s", rootKey, k)
		}
		if inpMap, ok := inp[k].(map[string]interface{}); ok {
			if err := parse(inpMap, out, localkey, h); err != nil {
				return err
			}
		} else if inpArr, ok := inp[k].([]interface{}); ok {
			var strArr = make([]string, len(inpArr))
			for idx, entry := range inpArr {
				strArr[idx] = fmt.Sprintf("%+v", entry)
			}
			out[localkey] = strings.Join(strArr, ",")
			h.Write([]byte(fmt.Sprintf("%s_%v", localkey, out[localkey])))
		} else {
			out[localkey] = fmt.Sprintf("%+v", inp[k])
			h.Write([]byte(fmt.Sprintf("%s_%v", localkey, inp[k])))
		}
	}
	return nil
}
