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

package error

import (
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transformations   = monitoring.NewInt(Metrics, "transformations")
	errorCounter      = monitoring.NewInt(Metrics, "errors")
	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")
	processorEntry    = common.MapStr{"name": processorName, "event": errorDocType}
)

func DecodePayload(raw map[string]interface{}) ([]transform.Eventable, error) {
	if raw == nil {
		return nil, nil
	}

	var err error
	decoder := utility.ManualDecoder{}
	errs := decoder.InterfaceArr(raw, "errors")

	events := make([]transform.Eventable, len(errs))
	for idx, errData := range errs {
		events[idx], err = DecodeEvent(errData, err)
	}
	return events, err
}
