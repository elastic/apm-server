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

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	v2 "github.com/elastic/apm-server/model/modeldecoder/v2"
	"github.com/elastic/apm-server/publish"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.profile")
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

// RequestMetadataFunc is a function type supplied to Handler for extracting
// metadata from the request. This is used for conditionally injecting the
// source IP address as `client.ip` for RUM.
type RequestMetadataFunc func(*request.Context) model.Metadata

// Handler returns a request.Handler for managing profile requests.
func Handler(requestMetadataFunc RequestMetadataFunc, processor model.BatchProcessor) request.Handler {
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

		var totalLimitRemaining int64 = profileContentLengthLimit
		var profiles []*pprof_profile.Profile
		var profileMetadata model.Metadata
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
				dec := decoder.NewJSONDecoder(r)
				metadata := requestMetadataFunc(c)
				if err := v2.DecodeMetadata(dec, &metadata); err != nil {
					if r.N < 0 {
						return nil, requestError{
							id:  request.IDResponseErrorsRequestTooLarge,
							err: err,
						}
					}
					if _, ok := err.(modeldecoder.ValidationError); ok {
						return nil, requestError{
							id:  request.IDResponseErrorsValidate,
							err: errors.Wrap(err, "invalid metadata"),
						}
					}
					return nil, requestError{
						id:  request.IDResponseErrorsDecode,
						err: errors.Wrap(err, "invalid metadata"),
					}
				}
				profileMetadata = metadata

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

		modelProfiles := make([]*model.PprofProfile, len(profiles))
		for i, p := range profiles {
			modelProfiles[i] = &model.PprofProfile{
				Metadata: profileMetadata,
				Profile:  p,
			}
		}

		if err := processor.ProcessBatch(c.Request.Context(), &model.Batch{Profiles: modelProfiles}); err != nil {
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
		return &result{Accepted: len(modelProfiles)}, nil
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
