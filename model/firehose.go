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

const (
	FirehoseLogDataset = "firehose.log"
)

// FirehoseProcessor is the Processor value that should be assigned to firehose events.
var FirehoseProcessor = Processor{Name: "firehose", Event: "log"}

// Firehose holds a firehose sample.
type Firehose struct {
	Message         string
	ARN             string
	ForwardedServer string
}

func (f *Firehose) setFields(fields *mapStr) {
	var firehoseFields mapStr
	firehoseFields.maybeSetString("arn", f.ARN)
	firehoseFields.maybeSetString("forwarded_server", f.ForwardedServer)
	fields.set("firehose", common.MapStr(firehoseFields))
	fields.set("message", f.Message)
}
