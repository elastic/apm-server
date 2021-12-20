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

package adaptive

const (
	// DecisionNone is used when no decision is taken.
	DecisionNone = iota

	// DecisionDownsample should be used to signal that downsampling should be
	// applied where needed.
	DecisionDownsample

	// DecisionUpsample should be used to signal that upsampling should be
	// applied where needed. Not implemented.
	DecisionUpsample

	// DecisionIndexerDownscale is not implemented.
	DecisionIndexerDownscale

	// DecisionIndexerUpscale should be used to signal that the number of
	// available bulk indexers should be increased in the modelindexer.Indexer.
	DecisionIndexerUpscale
)

// DecisionType represents the type up/down scaling decision taken.
type DecisionType uint8

// Decision represents a up/down scaling decision.
type Decision struct {
	// Type holds an adaptive decision type. The package constants should be
	// used to avoid mistakes.
	Type DecisionType

	// Factor represents a percentage which will be used to modify a value when
	// an adaptive action is performed.
	Factor float64
}
