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

package modelprocessor

import (
	"context"

	"github.com/elastic/apm-data/model/modelpb"
)

// SetErrorMessage is a modelpb.BatchProcessor that sets the APMEvent.Message
// field for error events.
type SetErrorMessage struct{}

// ProcessBatch sets the message for errors.
func (s SetErrorMessage) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	for i := range *b {
		event := (*b)[i]
		if event.Error != nil {
			event.Message = s.setErrorMessage(event)
		}
	}
	return nil
}

func (s SetErrorMessage) setErrorMessage(event *modelpb.APMEvent) string {
	if msg := event.GetError().GetLog().GetMessage(); msg != "" {
		return msg
	}
	if msg := event.GetError().GetException().GetMessage(); msg != "" {
		return msg
	}
	return ""
}
