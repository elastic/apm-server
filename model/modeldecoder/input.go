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

package modeldecoder

import (
	"time"

	"github.com/elastic/apm-server/model"
)

// Input holds the input required for decoding an event.
type Input struct {
	// RequestTime is the time at which the event was received
	// by the server. This is used to set the timestamp for
	// events sent by RUM.
	RequestTime time.Time

	// Metadata holds metadata that may be added to the event.
	Metadata model.Metadata

	// Config holds configuration for decoding.
	//
	// TODO(axw) define a Decoder type which encapsulates
	// static configuration defined in one location, removing
	// the possibility of inconsistent configuration.
	Config Config
}

// Config holds static configuration which applies to all decoding.
type Config struct {
	Experimental bool
}
