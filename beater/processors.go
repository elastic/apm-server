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

package beater

import (
	"context"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/ecs/code/go/ecs"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/model"
)

const (
	rateLimitTimeout = time.Second
)

// authorizeEventIngestProcessor is a model.BatchProcessor that checks that the
// client is authorized to ingest events for the given agent and service name.
func authorizeEventIngestProcessor(ctx context.Context, batch *model.Batch) error {
	for _, event := range *batch {
		if err := auth.Authorize(ctx, auth.ActionEventIngest, auth.Resource{
			AgentName:   event.Agent.Name,
			ServiceName: event.Service.Name,
		}); err != nil {
			return err
		}
	}
	return nil
}

// rateLimitBatchProcessor is a model.BatchProcessor that rate limits based on
// the batch size. This will be invoked after decoding events, but before sending
// on to the libbeat publisher.
func rateLimitBatchProcessor(ctx context.Context, batch *model.Batch) error {
	if limiter, ok := ratelimit.FromContext(ctx); ok {
		ctx, cancel := context.WithTimeout(ctx, rateLimitTimeout)
		defer cancel()
		if err := limiter.WaitN(ctx, len(*batch)); err != nil {
			return ratelimit.ErrRateLimitExceeded
		}
	}
	return nil
}

// ecsVersionBatchProcessor is a model.BatchProcessor that sets the ECSVersion
// field of each event to the ECS library version.
func ecsVersionBatchProcessor(ctx context.Context, b *model.Batch) error {
	for i := range *b {
		event := &(*b)[i]
		event.ECSVersion = ecs.Version
	}
	return nil
}

// newObserverBatchProcessor returns a model.BatchProcessor that sets observer
// fields from info.
func newObserverBatchProcessor(info beat.Info) model.ProcessBatchFunc {
	var versionMajor int
	if version, err := common.NewVersion(info.Version); err == nil {
		versionMajor = version.Major
	}
	return func(ctx context.Context, b *model.Batch) error {
		for i := range *b {
			observer := &(*b)[i].Observer
			observer.EphemeralID = info.EphemeralID.String()
			observer.Hostname = info.Hostname
			observer.ID = info.ID.String()
			if info.Name != info.Hostname {
				observer.Name = info.Name
			}
			observer.Type = info.Beat
			observer.Version = info.Version
			observer.VersionMajor = versionMajor
		}
		return nil
	}
}
