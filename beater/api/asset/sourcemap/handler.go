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

package sourcemap

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.elastic.co/apm"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/asset"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.sourcemap")
)

// RequestDecoder is the type for a function that decodes sourcemap data from an http.Request.
type RequestDecoder func(req *http.Request) (map[string]interface{}, error)

// Handler returns a request.Handler for managing asset requests.
func Handler(dec RequestDecoder, processor asset.Processor, report publish.Reporter) request.Handler {
	return func(c *request.Context) {
		if c.Request.Method != "POST" {
			c.Result.SetDefault(request.IDResponseErrorsMethodNotAllowed)
			c.Write()
			return
		}

		data, err := dec(c.Request)
		if err != nil {
			if strings.Contains(err.Error(), request.MapResultIDToStatus[request.IDResponseErrorsRequestTooLarge].Keyword) {
				c.Result.SetWithError(request.IDResponseErrorsRequestTooLarge, err)
			} else {
				c.Result.SetWithError(request.IDResponseErrorsDecode, err)
			}
			c.Write()
			return
		}

		if err = processor.Validate(data); err != nil {
			c.Result.SetWithError(request.IDResponseErrorsValidate, err)
			c.Write()
			return
		}

		transformables, err := processor.Decode(data)
		if err != nil {
			c.Result.SetWithError(request.IDResponseErrorsDecode, err)
			c.Write()
			return
		}

		req := publish.PendingReq{Transformables: transformables}
		span, ctx := apm.StartSpan(c.Request.Context(), "Send", "Reporter")
		defer span.End()
		req.Trace = !span.Dropped()

		if err = report(ctx, req); err != nil {
			if err == publish.ErrChannelClosed {
				c.Result.SetWithError(request.IDResponseErrorsShuttingDown, err)
			} else {
				c.Result.SetWithError(request.IDResponseErrorsFullQueue, err)
			}
			c.Write()
		}

		c.Result.SetDefault(request.IDResponseValidAccepted)
		c.Write()
	}
}

func DecodeSourcemapFormData(req *http.Request) (map[string]interface{}, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	file, _, err := req.FormFile("sourcemap")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sourcemapBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"sourcemap":       string(sourcemapBytes),
		"service_name":    req.FormValue("service_name"),
		"service_version": req.FormValue("service_version"),
		"bundle_filepath": utility.CleanUrlPath(req.FormValue("bundle_filepath")),
	}

	return payload, nil
}
