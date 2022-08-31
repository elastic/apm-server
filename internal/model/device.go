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

import (
	"github.com/elastic/elastic-agent-libs/mapstr"
)

type Device struct {
	// ID holds the unique identifier of a device.
	ID string

	// Architecture holds the host machine architecture.
	Model Model

	// Manufacturer the vendor name of the device manufacturer.
	Manufacturer string
}

type Model struct {
	// Name holds the human readable marketing name of the device model.
	Name string

	// Identifier holds the machine readable identifier of the device model.
	Identifier string
}

func (d *Device) fields() mapstr.M {
	if d == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("id", d.ID)

	var model mapStr
	model.maybeSetString("name", d.Model.Name)
	model.maybeSetString("identifier", d.Model.Identifier)
	fields.maybeSetMapStr("model", mapstr.M(model))

	fields.maybeSetString("manufacturer", d.Manufacturer)
	return mapstr.M(fields)
}
