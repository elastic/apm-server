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
	"github.com/elastic/beats/v7/libbeat/common"
)

type Network struct {
	EffectiveType string
	RoundTripTime int64
	Downlink      float64
	DownlinkMax   float64
	PhysicalLayer string
}

func (nw *Network) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("physical", nw.PhysicalLayer)
	fields.maybeSetFloat64("downlink", nw.Downlink)
	fields.maybeSetFloat64("downlink_max", nw.DownlinkMax)
	fields.maybeSetString("effective_type", nw.EffectiveType)
	fields.maybeSetInt64("rtt", nw.RoundTripTime)
	return common.MapStr(fields)
}
