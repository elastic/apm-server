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

package codec

import "fmt"

// vtprotoMessage expects a struct to have MarshalVT() and UnmarshalVT() methods
// that coverts data between byte slices and the desired format
type vtprotoMessage interface {
	MarshalVT() ([]byte, error)
	UnmarshalVT([]byte) error
}

// VTProto is a composite of Encoder and Decoder
type VTProto struct{}

// Encode encodes vtprotoMessage type into byte slice
func (v VTProto) Encode(in any) ([]byte, error) {
	vt, ok := in.(vtprotoMessage)
	if !ok {
		return nil, fmt.Errorf("failed to encode, message is %T (missing vtprotobuf helpers)", in)
	}
	return vt.MarshalVT()
}

// Decode decodes a byte slice into vtprotoMessage type.
func (v VTProto) Decode(in []byte, out any) error {
	vt, ok := out.(vtprotoMessage)
	if !ok {
		return fmt.Errorf("failed to decode, message is %T (missing vtprotobuf helpers)", out)
	}
	return vt.UnmarshalVT(in)
}
