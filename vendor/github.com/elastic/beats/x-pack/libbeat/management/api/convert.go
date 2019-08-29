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

package api

import (
	"fmt"
	"strings"
)

type converter func(map[string]interface{}) (map[string]interface{}, error)

var mapper = map[string]converter{
	".inputs":  noopConvert,
	".modules": convertMultiple,
	"output":   convertSingle,
}

var errSubTypeNotFound = fmt.Errorf("'%s' key not found", subTypeKey)

var (
	subTypeKey = "_sub_type"
	moduleKey  = "module"
)

func selectConverter(t string) converter {
	for k, v := range mapper {
		if strings.Index(t, k) > -1 {
			return v
		}
	}
	return noopConvert
}

func convertSingle(m map[string]interface{}) (map[string]interface{}, error) {
	subType, err := extractSubType(m)
	if err != nil {
		return nil, err
	}

	delete(m, subTypeKey)
	newMap := map[string]interface{}{subType: m}
	return newMap, nil
}

func convertMultiple(m map[string]interface{}) (map[string]interface{}, error) {
	subType, err := extractSubType(m)
	if err != nil {
		return nil, err
	}

	v, ok := m[moduleKey]

	if ok && v != subType {
		return nil, fmt.Errorf("module key already exist in the raw document and doesn't match the 'sub_type', expecting '%s' and received '%s", subType, v)
	}

	m[moduleKey] = subType
	delete(m, subTypeKey)
	return m, nil
}

func noopConvert(m map[string]interface{}) (map[string]interface{}, error) {
	return m, nil
}

func extractSubType(m map[string]interface{}) (string, error) {
	subType, ok := m[subTypeKey]
	if !ok {
		return "", errSubTypeNotFound
	}

	k, ok := subType.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for `sub_type`, expecting a string received %T", subType)
	}
	return k, nil
}
