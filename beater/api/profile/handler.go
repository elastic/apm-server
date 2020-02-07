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
	"mime"
	"net/http"

	pprof_profile "github.com/google/pprof/profile"
	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/profile"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.profile", monitoring.PublishExpvar)
)

const (
	pprofMediaType    = "application/x-protobuf"
	metadataMediaType = "application/json"
	requestMediaType  = "multipart/form-data"

	// value for the "messagetype" param
	pprofMessageType = "perftools.profiles.Profile"

	metadataContentLengthLimit = 10 * 1024
	profileContentLengthLimit  = 10 * 1024 * 1024
)

// Handler returns a request.Handler for managing profile requests.
func Handler(dec decoder.ReqDecoder, report publish.Reporter) request.Handler {
	handle := func(c *request.Context) (*result, error) {
		if c.Request.Method != http.MethodPost {
			return nil, requestError{
				id:  request.IDResponseErrorsMethodNotAllowed,
				err: errors.New("only POST requests are supported"),
			}
		}
		if _, err := validateContentType(c.Request.Header, requestMediaType); err != nil {
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

		var meta metadata.Metadata
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
				if _, err := validateContentType(http.Header(part.Header), metadataMediaType); err != nil {
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
				decodedMeta, err := metadata.DecodeMetadata(raw)
				if err != nil {
					return nil, requestError{
						id:  request.IDResponseErrorsDecode,
						err: errors.Wrap(err, "failed to decode metadata"),
					}
				}
				meta = *decodedMeta

			case "profile":
				params, err := validateContentType(http.Header(part.Header), pprofMediaType)
				if err != nil {
					return nil, requestError{
						id:  request.IDResponseErrorsValidate,
						err: errors.Wrap(err, "invalid profile"),
					}
				}
				if v, ok := params["messagetype"]; ok && v != pprofMessageType {
					// If messagetype is specified, require that it matches.
					// Otherwise we assume that it's pprof, and we'll error
					// out below if it doesn't decode.
					return nil, requestError{
						id:  request.IDResponseErrorsValidate,
						err: errors.Wrapf(err, "expected messagetype %q, got %q", pprofMessageType, v),
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

		transformables := make([]publish.Transformable, len(profiles))
		for i, p := range profiles {
			transformables[i] = profile.PprofProfile{Profile: p, Metadata: meta}
		}

		if err := report(c.Request.Context(), publish.PendingReq{
			Transformables: transformables,
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

func validateContentType(header http.Header, expectedMediatype string) (params map[string]string, err error) {
	mediatype, params, err := mime.ParseMediaType(header.Get(headers.ContentType))
	if err != nil {
		return nil, err
	}
	if mediatype != expectedMediatype {
		return nil, fmt.Errorf("invalid content type %q, expected %q", mediatype, expectedMediatype)
	}
	return params, nil
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
