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
	"io"
	"net/http"
	"strings"

	pprof_profile "github.com/google/pprof/profile"
	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/profile"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.MonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.profile", monitoring.PublishExpvar)
)

const (
	// TODO(axw) include messageType in pprofContentType; needs fix in agent
	pprofContentType    = "application/x-protobuf"
	metadataContentType = "application/json"
	requestContentType  = "multipart/form-data"

	metadataContentLengthLimit = 10 * 1024
	profileContentLengthLimit  = 10 * 1024 * 1024
)

// Handler returns a request.Handler for managing profile requests.
func Handler(
	dec decoder.ReqDecoder,
	transformConfig transform.Config,
	report publish.Reporter,
) request.Handler {
	handle := func(c *request.Context) (*result, error) {
		if c.Request.Method != http.MethodPost {
			return nil, requestError{
				id:  request.IDResponseErrorsMethodNotAllowed,
				err: errors.New("only POST requests are supported"),
			}
		}
		if err := validateContentType(c.Request.Header, requestContentType); err != nil {
			return nil, requestError{
				id:  request.IDResponseErrorsValidate,
				err: err,
			}
		}

		ok := c.RateLimiter == nil || c.RateLimiter.Allow()
		if !ok {
			return nil, requestError{
				id:  request.IDResponseErrorsRateLimit,
				err: errors.New("rate limit exceeded"),
			}
		}

		// Extract metadata from the request, such as the remote address.
		reqMeta, err := dec(c.Request)
		if err != nil {
			return nil, requestError{
				id:  request.IDResponseErrorsDecode,
				err: errors.Wrap(err, "failed to decode request metadata"),
			}
		}

		tctx := &transform.Context{
			RequestTime: utility.RequestTime(c.Request.Context()),
			Config:      transformConfig,
		}

		var totalLimitRemaining int64 = profileContentLengthLimit
		var profiles []*pprof_profile.Profile
		mr, err := c.Request.MultipartReader()
		if err != nil {
			return nil, err
		}
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}

			switch part.FormName() {
			case "metadata":
				if err := validateContentType(http.Header(part.Header), metadataContentType); err != nil {
					return nil, requestError{
						id:  request.IDResponseErrorsValidate,
						err: errors.Wrap(err, "invalid metadata"),
					}
				}
				r := &decoder.LimitedReader{R: part, N: metadataContentLengthLimit}
				raw, err := decoder.DecodeJSONData(r)
				if err != nil {
					if r.N < 0 {
						return nil, requestError{
							id:  request.IDResponseErrorsRequestTooLarge,
							err: err,
						}
					}
					return nil, requestError{
						id:  request.IDResponseErrorsDecode,
						err: errors.Wrap(err, "failed to decode metadata JSON"),
					}
				}
				for k, v := range reqMeta {
					utility.InsertInMap(raw, k, v.(map[string]interface{}))
				}
				if err := validation.Validate(raw, metadata.ModelSchema()); err != nil {
					return nil, requestError{
						id:  request.IDResponseErrorsValidate,
						err: errors.Wrap(err, "invalid metadata"),
					}
				}
				metadata, err := metadata.DecodeMetadata(raw)
				if err != nil {
					return nil, requestError{
						id:  request.IDResponseErrorsDecode,
						err: errors.Wrap(err, "failed to decode metadata"),
					}
				}
				tctx.Metadata = *metadata

			case "profile":
				if err := validateContentType(http.Header(part.Header), pprofContentType); err != nil {
					return nil, requestError{
						id:  request.IDResponseErrorsValidate,
						err: errors.Wrap(err, "invalid profile"),
					}
				}
				r := &decoder.LimitedReader{R: part, N: totalLimitRemaining}
				profile, err := pprof_profile.Parse(r)
				if err != nil {
					if r.N < 0 {
						return nil, requestError{
							id:  request.IDResponseErrorsRequestTooLarge,
							err: err,
						}
					}
					return nil, requestError{
						id:  request.IDResponseErrorsDecode,
						err: errors.Wrap(err, "failed to decode profile"),
					}
				}
				profiles = append(profiles, profile)
				totalLimitRemaining = r.N
			}
		}

		transformables := make([]transform.Transformable, len(profiles))
		for i, p := range profiles {
			transformables[i] = profile.PprofProfile{Profile: p}
		}

		if err := report(c.Request.Context(), publish.PendingReq{
			Transformables: transformables,
			Tcontext:       tctx,
		}); err != nil {
			switch err {
			case publish.ErrChannelClosed:
				return nil, requestError{
					id:  request.IDResponseErrorsShuttingDown,
					err: errors.New("server is shutting down"),
				}
			case publish.ErrFull:
				return nil, requestError{
					id:  request.IDResponseErrorsFullQueue,
					err: err,
				}
			}
			return nil, err
		}
		return &result{Accepted: len(transformables)}, nil
	}
	return func(c *request.Context) {
		result, err := handle(c)
		if err != nil {
			switch err := err.(type) {
			case requestError:
				c.Result.SetWithError(err.id, err)
			default:
				c.Result.SetWithError(request.IDResponseErrorsInternal, err)
			}
		} else {
			c.Result.SetWithBody(request.IDResponseValidAccepted, result)
		}
		c.Write()
	}
}

func validateContentType(header http.Header, contentType string) error {
	got := header.Get(headers.ContentType)
	if !strings.Contains(got, contentType) {
		return fmt.Errorf("invalid content type %q, expected %q", got, contentType)
	}
	return nil
}

type result struct {
	Accepted int `json:"accepted"`
}

type requestError struct {
	id  request.ResultID
	err error
}

func (e requestError) Error() string {
	return e.err.Error()
}
