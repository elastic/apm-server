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

package convert

import (
	"bytes"
	"encoding/json"
	"io"
)

// FromReader reads the given reader into the given interface
func FromReader(r io.ReadCloser, i interface{}) error {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return FromBytes(buf.Bytes(), i, err)
}

// FromBytes reads the given byte slice into the given interface
func FromBytes(bs []byte, i interface{}, err error) error {
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, i)
}

// ToReader converts a marshall-able interface into a reader
func ToReader(i interface{}) io.Reader {
	b, _ := json.Marshal(i)
	return bytes.NewReader(b)
}
