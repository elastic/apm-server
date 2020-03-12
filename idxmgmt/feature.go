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

package idxmgmt

import (
	libidxmgmt "github.com/elastic/beats/v7/libbeat/idxmgmt"
)

type feature struct {
	enabled, overwrite, load, supported bool

	warn string
	info string
	err  error
}

func newFeature(enabled, overwrite, load, supported bool, mode libidxmgmt.LoadMode) feature {
	if mode == libidxmgmt.LoadModeUnset {
		mode = libidxmgmt.LoadModeDisabled
	}
	if mode >= libidxmgmt.LoadModeOverwrite {
		overwrite = true
	}
	if mode == libidxmgmt.LoadModeForce {
		load = true
	}
	if !supported {
		enabled = false
	}
	load = load && mode.Enabled()
	return feature{
		enabled:   enabled,
		overwrite: overwrite,
		load:      load,
		supported: supported}
}

func (f *feature) warning() string {
	return f.warn
}

func (f *feature) information() string {
	return f.info
}

func (f *feature) error() error {
	return f.err
}
