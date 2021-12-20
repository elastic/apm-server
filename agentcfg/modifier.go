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
	"context"
)

// Adapter defines the behavior of adapters.
type Adapter interface {
	Adapt(*Result)
}

// Modifier wraps a fetcher and runs a series of config adapters when the
// retrieved agent configuration is found.
type Modifier struct {
	f Fetcher

	adapters   []Adapter
	adaptEmpty bool
}

// NewModifier returns a new instance of Modifier.
func NewModifier(f Fetcher, adaptEmpty bool, adapters ...Adapter) *Modifier {
	return &Modifier{f: f, adapters: adapters, adaptEmpty: adaptEmpty}
}

// Fetch wraps the fetcher's Fetch() and executes all the adapters on the
// retrieved configuration.
func (f *Modifier) Fetch(ctx context.Context, query Query) (Result, error) {
	res, _ := f.f.Fetch(ctx, query)
	if f.adaptEmpty && len(res.Source.Settings) == 0 {
		res.Source.Agent = query.Service.Name
	}
	for _, adapter := range f.adapters {
		adapter.Adapt(&res)
	}
	return res, nil
}
