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

package sourcemap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	tests := []struct {
		Msg     string
		Kind    Enum
		OutMsg  string
		OutKind string
	}{
		{OutMsg: "", OutKind: ""},
		{Msg: "Init failed", Kind: InitError, OutMsg: "Init failed", OutKind: "InitError"},
		{Msg: "Access failed", Kind: AccessError, OutMsg: "Access failed", OutKind: "AccessError"},
		{Msg: "Map failed", Kind: MapError, OutMsg: "Map failed", OutKind: "MapError"},
		{Msg: "Parse failed", Kind: ParseError, OutMsg: "Parse failed", OutKind: "ParseError"},
		{Msg: "Key error", Kind: KeyError, OutMsg: "Key error", OutKind: "KeyError"},
	}
	for idx, test := range tests {
		err := Error{Msg: test.Msg, Kind: test.Kind}
		assert.Equal(t, test.OutMsg, err.Msg,
			fmt.Sprintf("(%v): Expected <%v>, Received <%v> ", idx, test.OutMsg, err.Msg))
		assert.Equal(t, test.OutKind, string(err.Kind),
			fmt.Sprintf("(%v): Expected <%v>, Received <%v> ", idx, test.OutKind, err.Kind))
	}
}
