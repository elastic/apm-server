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

package model

import "github.com/elastic/beats/v7/libbeat/common"

// Session holds information about a group of related transactions, such as
// a sequence of web interactions.
type Session struct {
	// ID holds a session ID for grouping a set of related transactions.
	ID string

	// Sequence holds an optional sequence number for a transaction
	// within a session. Sequence is ignored if it is zero or if
	// ID is empty.
	Sequence int
}

func (s *Session) fields() common.MapStr {
	if s.ID == "" {
		return nil
	}
	out := common.MapStr{"id": s.ID}
	if s.Sequence > 0 {
		out["sequence"] = s.Sequence
	}
	return out
}
