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

type Network struct {
	// Connection holds information about a network connection.
	Connection NetworkConnection

	// Carrier holds information about a connection carrier.
	Carrier NetworkCarrier
}

type NetworkConnection struct {
	// Type holds the connection type category,
	// e.g. "wifi", "wired", and "cell".
	Type string

	// Subtype holds more details of the connection type,
	// specific to the connection type category.
	//
	// For example, if ConnectionType is "cell" then ConnectionSubtype
	// may hold the cell technology, e.g. "LTE", or "GRPS".
	Subtype string
}

type NetworkCarrier struct {
	// Name holds the carrier's name.
	Name string

	// MCC holds the carrier's mobile country code.
	MCC string

	// MNC holds the carrier's mobile network code.
	MNC string

	// ICC holds the carrier's ISO 3166-1 alpha-2 2-character country code.
	ICC string
}

func (n *Network) fields() common.MapStr {
	var network mapStr
	network.maybeSetMapStr("connection", n.Connection.fields())
	network.maybeSetMapStr("carrier", n.Carrier.fields())
	return common.MapStr(network)
}

func (c *NetworkConnection) fields() common.MapStr {
	var connection mapStr
	connection.maybeSetString("type", c.Type)
	connection.maybeSetString("subtype", c.Subtype)
	return common.MapStr(connection)
}

func (c *NetworkCarrier) fields() common.MapStr {
	var carrier mapStr
	carrier.maybeSetString("mcc", c.MCC)
	carrier.maybeSetString("mnc", c.MNC)
	carrier.maybeSetString("icc", c.ICC)
	carrier.maybeSetString("name", c.Name)
	return common.MapStr(carrier)
}
