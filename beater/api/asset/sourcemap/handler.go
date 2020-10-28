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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.elastic.co/apm"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.sourcemap")

	decodingCount = monitoring.NewInt(registry, "decoding.count")
	decodingError = monitoring.NewInt(registry, "decoding.errors")
	validateCount = monitoring.NewInt(registry, "validation.count")
	validateError = monitoring.NewInt(registry, "validation.errors")
)

// Handler returns a request.Handler for managing asset requests.
func Handler(report publish.Reporter) request.Handler {
	return func(c *request.Context) {
		if c.Request.Method != "POST" {
			c.Result.SetDefault(request.IDResponseErrorsMethodNotAllowed)
			c.Write()
			return
		}
		var smap model.Sourcemap
		decodingCount.Inc()
		if err := decode(c.Request, &smap); err != nil {
			decodingError.Inc()
			if strings.Contains(err.Error(), request.MapResultIDToStatus[request.IDResponseErrorsRequestTooLarge].Keyword) {
				c.Result.SetWithError(request.IDResponseErrorsRequestTooLarge, err)
			} else {
				c.Result.SetWithError(request.IDResponseErrorsDecode, err)
			}
			c.Write()
			return
		}
		validateCount.Inc()
		if err := validate(smap); err != nil {
			validateCount.Inc()
			c.Result.SetWithError(request.IDResponseErrorsValidate, err)
			c.Write()
			return
		}

		req := publish.PendingReq{Transformables: []transform.Transformable{&smap}}
		span, ctx := apm.StartSpan(c.Request.Context(), "Send", "Reporter")
		defer span.End()
		req.Trace = !span.Dropped()
		if err := report(ctx, req); err != nil {
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

func decode(req *http.Request, smap *model.Sourcemap) error {
	if !strings.Contains(req.Header.Get("Content-Type"), "multipart/form-data") {
		return fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}
	file, _, err := req.FormFile("sourcemap")
	if err != nil {
		return err
	}
	defer file.Close()
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	smap.Sourcemap = string(bytes)
	smap.BundleFilepath = utility.CleanUrlPath(req.FormValue("bundle_filepath"))
	smap.ServiceName = req.FormValue("service_name")
	smap.ServiceVersion = req.FormValue("service_version")
	return nil
}

func validate(smap model.Sourcemap) error {
	// ensure all information is given
	if smap.BundleFilepath == "" || smap.ServiceName == "" || smap.ServiceVersion == "" {
		return errors.New("error validating sourcemap: bundle_filepath, service_name and service_version must be sent")
	}
	if smap.Sourcemap == "" {
		return errors.New(`error validating sourcemap: expected sourcemap to be sent as string, but got null`)
	}
	return nil
}
