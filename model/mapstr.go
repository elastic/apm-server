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

type mapStr common.MapStr

func (m *mapStr) set(k string, v interface{}) {
	if *m == nil {
		*m = make(mapStr)
	}
	(*m)[k] = v
}

func (m *mapStr) maybeSetString(k, v string) bool {
	if v != "" {
		m.set(k, v)
		return true
	}
	return false
}

func (m *mapStr) maybeSetBool(k string, v *bool) bool {
	if v != nil {
		m.set(k, *v)
		return true
	}
	return false
}

func (m *mapStr) maybeSetIntptr(k string, v *int) bool {
	if v != nil {
		m.set(k, *v)
		return true
	}
	return false
}

func (m *mapStr) maybeSetFloat64ptr(k string, v *float64) bool {
	if v != nil {
		m.set(k, *v)
		return true
	}
	return false
}

func (m *mapStr) maybeSetMapStr(k string, v common.MapStr) bool {
	if len(v) > 0 {
		m.set(k, v)
		return true
	}
	return false
}

func (m mapStr) delete(k string) mapStr {
	fs := common.MapStr(m)
	fs.Delete(k)
	return mapStr(fs)
}
