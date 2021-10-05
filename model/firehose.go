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
	"strings"

	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	FirehoseDataset = "firehose"

	arnDelimiter     = ":"
	arnSections      = 6
	sectionPartition = 1
	sectionService   = 2
	sectionRegion    = 3
	sectionAccountID = 4
	sectionResource  = 5
)

// Firehose holds a firehose sample.
type Firehose struct {
	Message string
	ARN     string
}

// ARN struct separate the Amazon Resource Name into individual fields.
type ARN struct {
	Partition string
	Service   string
	Region    string
	AccountID string
	Resource  string
}

func (f *Firehose) setFields(fields *mapStr) {
	var firehoseFields mapStr
	fields.set("firehose", common.MapStr(firehoseFields))
	fields.set("message", f.Message)

	arn := parseARN(f.ARN)
	fields.set("cloud.origin.region", arn.Region)
	fields.set("cloud.origin.account.id", arn.AccountID)
	fields.set("agent.id", f.ARN)
	fields.set("agent.name", arn.Resource)
}

func parseARN(arn string) ARN {
	// arn example for firehose:
	// arn:aws:firehose:us-east-1:123456789:deliverystream/vpc-flow-log-stream-http-endpoint
	sections := strings.SplitN(arn, arnDelimiter, arnSections)
	if len(sections) != arnSections {
		return ARN{}
	}
	return ARN{
		Partition: sections[sectionPartition],
		Service:   sections[sectionService],
		Region:    sections[sectionRegion],
		AccountID: sections[sectionAccountID],
		Resource:  sections[sectionResource],
	}
}
