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

package main

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-agent-libs/version"
)

// getCommonPipelines returns pipelines that may be inlined into our data stream ingest pipelines.
//
// To use a common pipeline, define a "pipeline" processor with the "name" property set to one of
// the common pipelines. e.g.
//
//	processors:
//	  - ...
//	  - pipeline:
//	      name: observer_version
//	  - ...
func getCommonPipeline(name string, version *version.V) []map[string]interface{} {
	commonPipelines := map[string][]map[string]interface{}{
		"observer_version": getObserverVersionPipeline(version),
		"observer_ids":     observerIDsPipeline,
		"ecs_version":      ecsVersionPipeline,
		"user_agent":       userAgentPipeline,
		"process_ppid":     processPpidPipeline,
		"client_geoip":     clientGeoIPPipeline,
		"event_duration":   eventDurationPipeline,
	}
	return commonPipelines[name]
}

// observerVersionPipeline ensures the observer version (i.e. apm-server version) is
// no greater than the integration package version. The integration package version
// is always expected to be greater, so this allows us to better reason about and
// avoid version mismatch bugs.
func getObserverVersionPipeline(version *version.V) []map[string]interface{} {
	observerVersionCheckIf := fmt.Sprintf(
		"ctx.observer.version_major > %d || (ctx.observer.version_major == %d && ctx.observer.version_minor > %d)",
		version.Major, version.Major, version.Minor,
	)
	observerVersionCheckMessage := fmt.Sprintf(""+
		"Document produced by APM Server v{{{observer.version}}}, "+
		"which is newer than the installed APM integration (v%s). "+
		"The APM integration must be upgraded.",
		version,
	)

	return []map[string]interface{}{{
		// Parse observer.version into observer.version_major, observer.version_minor,
		// and observer.version_patch fields.
		"grok": map[string]interface{}{
			"field":               "observer.version",
			"pattern_definitions": map[string]string{"DIGITS": "(?:[0-9]+)"},
			"patterns": []string{"" +
				"%{DIGITS:observer.version_major:int}." +
				"%{DIGITS:observer.version_minor:int}." +
				"%{DIGITS:observer.version_patch:int}(?:[-+].*)?",
			},
		},
	}, {
		"fail": map[string]interface{}{
			"if":      observerVersionCheckIf,
			"message": observerVersionCheckMessage,
		},
	}, {
		// Remove observer.version_major, observer.version_minor and observer.version_patch fields introduced above,
		"remove": map[string]interface{}{
			"ignore_missing": true,
			"field":          []string{"observer.version_major", "observer.version_minor", "observer.version_patch"},
		},
	}}
}

var observerIDsPipeline = []map[string]interface{}{{
	"remove": map[string]interface{}{
		// Remove observer.id and observer.ephemeral_id.
		"field": []string{
			"observer.id",
			"observer.ephemeral_id",
		},
		"ignore_missing": true,
		"ignore_failure": true,
	},
}}

var ecsVersionPipeline = []map[string]interface{}{{
	"remove": map[string]interface{}{
		"field":          "ecs", // remove ecs.version
		"ignore_missing": true,
		"ignore_failure": true,
	},
}}

var userAgentPipeline = []map[string]interface{}{{
	"user_agent": map[string]interface{}{
		"field":          "user_agent.original",
		"target_field":   "user_agent",
		"ignore_missing": true,
		"ignore_failure": true,
	},
}}

var processPpidPipeline = []map[string]interface{}{{
	"rename": map[string]interface{}{
		"field":          "process.ppid",
		"target_field":   "process.parent.pid",
		"ignore_missing": true,
		"ignore_failure": true,
	},
}}

var clientGeoIPPipeline = []map[string]interface{}{{
	"geoip": map[string]interface{}{
		"field":          "client.ip",
		"target_field":   "client.geo",
		"ignore_missing": true,
		"database_file":  "GeoLite2-City.mmdb",
		"on_failure": []map[string]interface{}{{
			"remove": map[string]interface{}{
				"field":          "client.ip",
				"ignore_missing": true,
				"ignore_failure": true,
			},
		}},
	},
}}

// This pipeline translates `event.duration` (defaulting to zero if not
// found) to `transaction.duration.us` or `span.duration.us` depending on
// the event type, and then removes `event.duration`. Older versions of
// APM Server will send `<event>.duration.us`, in which case we skip this
// pipeline.
//
// TODO(axw) remove this pipeline when we are ready to migrate the UI to
// `event.duration`. See https://github.com/elastic/apm-server/issues/5999.
var eventDurationPipeline = []map[string]interface{}{{
	"script": map[string]interface{}{
		"if": "ctx.processor?.event != null && ctx.get(ctx.processor.event) != null && ctx.get(ctx.processor.event)?.duration == null",
		"source": strings.TrimSpace(`
def durationNanos = ctx.event?.duration ?: 0;
def eventType = ctx.processor.event;
ctx.get(ctx.processor.event).duration = ["us": (long)(durationNanos/1000)];
`),
	},
}, {
	"remove": map[string]interface{}{
		"field":          "event.duration",
		"ignore_missing": true,
		"ignore_failure": true,
	},
}}
