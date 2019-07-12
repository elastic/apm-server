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
	var doc Doc
	if len(in) == 0 {
		return &doc, nil
	}
	var tmp tmpDoc
	if err := json.Unmarshal(in, &tmp); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tmp.Source.Settings, &doc.Settings); err != nil {
		return nil, err
	}
	doc.ID = tmp.hash()
	return &doc, nil
}

type tmpDoc struct {
	ID     string `json:"_id"`
	Source struct {
		Settings json.RawMessage `json:"settings"`
	} `json:"_source"`
}

func (t *tmpDoc) hash() string {
	h := md5.New()
	h.Write([]byte(t.ID))
	h.Write([]byte(t.Source.Settings))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// UnmarshalJSON overrides default method to convert any JSON type to string
func (s *Settings) UnmarshalJSON(b []byte) error {
	in := make(map[string]interface{})
	out := make(map[string]string)
	err := json.Unmarshal(b, &in)
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	*s = out
	return err
}
