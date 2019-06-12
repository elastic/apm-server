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

	"github.com/go-sourcemap/sourcemap"
)

type Accessor interface {
	Fetch(id Id) (*sourcemap.Consumer, error)
	Remove(id Id)
}

type SmapAccessor struct {
	es    elasticsearch
	cache *cache
}

func NewSmapAccessor(config Config) (*SmapAccessor, error) {
	es, err := NewElasticsearch(config.ElasticsearchConfig, config.Index)
	if err != nil {
		return nil, err
	}
	cache, err := newCache(config.CacheExpiration)
	if err != nil {
		return nil, err
	}
	return &SmapAccessor{
		es:    es,
		cache: cache,
	}, nil
}

func (s *SmapAccessor) Fetch(id Id) (*sourcemap.Consumer, error) {
	if !id.Valid() {
		return nil, Error{
			Msg:  fmt.Sprintf("Sourcemap Key Error for %v", id.String()),
			Kind: KeyError,
		}
	}
	consumer, found := s.cache.fetch(id)
	if consumer != nil {
		// avoid fetching ES again when key was already queried
		// but no Sourcemap was found for it.
		return consumer, nil
	}
	if found {
		return nil, errSmapNotAvailable(id)
	}
	consumer, err := s.es.fetch(id)
	if err != nil {
		return nil, err
	}
	s.cache.add(id, consumer)
	if consumer == nil {
		return nil, errSmapNotAvailable(id)
	}
	return consumer, nil
}

func (s *SmapAccessor) Remove(id Id) {
	s.cache.remove(id)
}

func errSmapNotAvailable(id Id) Error {
	return Error{
		Msg:  fmt.Sprintf("No Sourcemap available for %v.", id.String()),
		Kind: MapError,
	}
}
