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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
)

func TestNewSmapMapper(t *testing.T) {
	mapper, err := NewSmapMapper(Config{})
	assert.Nil(t, mapper)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, InitError)

	mapper, err = NewSmapMapper(getFakeConfig())
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

func TestApply(t *testing.T) {
	mapper, err := NewSmapMapper(getFakeConfig())
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// error occurs
	mapping, err := mapper.Apply(Id{}, 0, 0)
	assert.Nil(t, mapping)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, KeyError)

	// no mapping found in sourcemap
	mapper.Accessor = &fakeAccessor{}
	mapping, err = mapper.Apply(Id{Path: "bundle.js.map"}, 0, 0)
	assert.Nil(t, mapping)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No Sourcemap found")
	assert.Equal(t, (err.(Error)).Kind, KeyError)

	// mapping found in minified sourcemap
	mapping, err = mapper.Apply(Id{Path: "bundle.js.map"}, 1, 7)
	assert.NoError(t, err)
	assert.NotNil(t, mapping)
	assert.Equal(t, "webpack:///bundle.js", mapping.Filename)
	assert.Equal(t, "", mapping.Function)
	assert.Equal(t, 1, mapping.Lineno)
	assert.Equal(t, 9, mapping.Colno)
	assert.Equal(t, "bundle.js.map", mapping.Path)
}

func TestSubSlice(t *testing.T) {
	src := []string{"a", "b", "c", "d", "e", "f"}
	for _, test := range []struct {
		start, end int
		rs         []string
	}{
		{2, 4, []string{"c", "d"}},
		{-1, 1, []string{"a"}},
		{4, 10, []string{"e", "f"}},
	} {
		assert.Equal(t, test.rs, subSlice(test.start, test.end, src))
	}

	for _, test := range []struct {
		start, end int
	}{
		{0, 1},
		{0, 0},
		{-1, 0},
	} {
		assert.Equal(t, []string{}, subSlice(test.start, test.end, []string{}))
	}
}

type fakeAccessor struct{}

func (ac *fakeAccessor) Fetch(smapId Id) (*sourcemap.Consumer, error) {
	current, _ := os.Getwd()
	path := filepath.Join(current, "../tests/data/sourcemap/", smapId.Path)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return sourcemap.Parse("", fileBytes)
}
func (ac *fakeAccessor) Remove(smapId Id) {}
