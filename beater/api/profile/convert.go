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

package profile

import (
	"fmt"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gofrs/uuid"
	"github.com/google/pprof/profile"

	"github.com/elastic/apm-server/model"
)

// appendProfileSampleBatch converts a pprof profile into a batch of model.ProfileSamples,
// and appends it to out.
func appendProfileSampleBatch(pp *profile.Profile, baseEvent model.APMEvent, out model.Batch) model.Batch {

	// Precompute value field names for use in each event.
	// TODO(axw) limit to well-known value names?
	baseEvent.Timestamp = time.Unix(0, pp.TimeNanos)
	valueFieldNames := make([]string, len(pp.SampleType))
	for i, sampleType := range pp.SampleType {
		sampleUnit := normalizeUnit(sampleType.Unit)
		// Go profiles report samples.count, Node.js profiles report sample.count.
		// We use samples.count for both so we can aggregate on one field.
		if sampleType.Type == "sample" || sampleType.Type == "samples" {
			valueFieldNames[i] = "samples.count"
		} else {
			valueFieldNames[i] = sampleType.Type + "." + sampleUnit
		}

	}

	// Generate a unique profile ID shared by all samples in the profile.
	// If we can't generate a UUID for whatever reason, omit the profile ID.
	var profileID string
	if uuid, err := uuid.NewV4(); err == nil {
		profileID = fmt.Sprintf("%x", uuid)
	}

	for _, sample := range pp.Sample {
		var stack []model.ProfileSampleStackframe
		if n := len(sample.Location); n > 0 {
			hash := xxhash.New()
			stack = make([]model.ProfileSampleStackframe, n)
			for i := len(sample.Location) - 1; i >= 0; i-- {
				loc := sample.Location[i]
				line := loc.Line[0] // aggregated at function level

				// NOTE(axw) Currently we hash the function names so that
				// we can aggregate stacks across multiple builds, or where
				// binaries are not reproducible.
				//
				// If we decide to identify stack traces and frames using
				// function addresses, then need to subtract the mapping's
				// start address to eliminate the effects of ASLR, i.e.
				//
				//     var buf [8]byte
				//     binary.BigEndian.PutUint64(buf[:], loc.Address-loc.Mapping.Start)
				//     hash.Write(buf[:])

				hash.WriteString(line.Function.Name)
				stack[i] = model.ProfileSampleStackframe{
					ID:       fmt.Sprintf("%x", hash.Sum(nil)),
					Function: line.Function.Name,
					Filename: line.Function.Filename,
					Line:     line.Line,
				}
			}
		}

		event := baseEvent
		event.Labels = event.Labels.Clone()
		if n := len(sample.Label); n > 0 {
			for k, v := range sample.Label {
				event.Labels[k] = v
			}
		}

		values := make(map[string]int64, len(sample.Value))
		for i, value := range sample.Value {
			values[valueFieldNames[i]] = value
		}

		event.ProfileSample = &model.ProfileSample{
			Duration:  time.Duration(pp.DurationNanos),
			ProfileID: profileID,
			Stack:     stack,
			Values:    values,
		}
		out = append(out, event)
	}
	return out
}

func normalizeUnit(unit string) string {
	switch unit {
	case "nanoseconds":
		unit = "ns"

	case "microseconds":
		unit = "us"
	}
	return unit
}
