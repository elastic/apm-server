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
	"context"
	"fmt"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gofrs/uuid"
	"github.com/google/pprof/profile"

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	profileProcessorName = "profile"
	profileDocType       = "profile"
	ProfilesDataset      = "apm.profiling"
)

var profileProcessorEntry = common.MapStr{
	"name":  profileProcessorName,
	"event": profileDocType,
}

// PprofProfile represents a resource profile.
type PprofProfile struct {
	Metadata Metadata
	Profile  *profile.Profile
}

// Transform transforms a Profile into a sequence of beat.Events: one per profile sample.
func (pp PprofProfile) Transform(ctx context.Context, cfg *transform.Config) []beat.Event {
	// Precompute value field names for use in each event.
	// TODO(axw) limit to well-known value names?
	profileTimestamp := time.Unix(0, pp.Profile.TimeNanos)
	valueFieldNames := make([]string, len(pp.Profile.SampleType))
	for i, sampleType := range pp.Profile.SampleType {
		sampleUnit := normalizeUnit(sampleType.Unit)
		valueFieldNames[i] = sampleType.Type + "." + sampleUnit
	}

	// Generate a unique profile ID shared by all samples in the profile.
	// If we can't generate a UUID for whatever reason, omit the profile ID.
	var profileID string
	if uuid, err := uuid.NewV4(); err == nil {
		profileID = fmt.Sprintf("%x", uuid)
	}

	// Profiles are stored in their own "metrics" data stream, with a data
	// set per service. This enables managing retention of profiling data
	// per-service, and indepedently of lower volume metrics.

	samples := make([]beat.Event, len(pp.Profile.Sample))
	for i, sample := range pp.Profile.Sample {
		profileFields := common.MapStr{}
		if profileID != "" {
			profileFields["id"] = profileID
		}
		if pp.Profile.DurationNanos > 0 {
			profileFields["duration"] = pp.Profile.DurationNanos
		}
		if len(sample.Location) > 0 {
			hash := xxhash.New()
			stack := make([]common.MapStr, len(sample.Location))
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
				fields := common.MapStr{
					"id":       fmt.Sprintf("%x", hash.Sum(nil)),
					"function": line.Function.Name,
				}
				if line.Function.Filename != "" {
					utility.Set(fields, "filename", line.Function.Filename)
					if line.Line > 0 {
						utility.Set(fields, "line", line.Line)
					}
				}
				stack[i] = fields
			}
			utility.Set(profileFields, "stack", stack)
			utility.Set(profileFields, "top", stack[0])
		}
		for i, v := range sample.Value {
			utility.Set(profileFields, valueFieldNames[i], v)
		}
		event := beat.Event{
			Timestamp: profileTimestamp,
			Fields: common.MapStr{
				"processor":    profileProcessorEntry,
				profileDocType: profileFields,
			},
		}
		if cfg.DataStreams {
			event.Fields[datastreams.TypeField] = datastreams.MetricsType
			dataset := fmt.Sprintf("%s.%s", ProfilesDataset, datastreams.NormalizeServiceName(pp.Metadata.Service.Name))
			event.Fields[datastreams.DatasetField] = dataset
		}
		var profileLabels common.MapStr
		if len(sample.Label) > 0 {
			profileLabels = make(common.MapStr)
			for k, v := range sample.Label {
				profileLabels[k] = v
			}
		}
		pp.Metadata.Set(event.Fields, profileLabels)
		samples[i] = event
	}
	return samples
}

func normalizeUnit(unit string) string {
	switch unit {
	case "nanoseconds":
		unit = "ns"
	}
	return unit
}
