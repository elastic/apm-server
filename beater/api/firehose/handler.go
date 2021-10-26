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

package firehose

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/common"
)

const dataset = "apm.firehose"

type record struct {
	Data string `json:"data"`
}

type firehose struct {
	RequestID string   `json:"requestId"`
	Timestamp int64    `json:"timestamp"`
	Records   []record `json:"records"`
}

type cloudwatchMetric struct {
	AccountID        string             `json:"account_id"`
	Dimensions       map[string]string  `json:"dimensions"`
	MetricName       string             `json:"metric_name"`
	MetricStreamName string             `json:"metric_stream_name"`
	Namespace        string             `json:"namespace"`
	Region           string             `json:"region"`
	Timestamp        int64              `json:"timestamp"`
	Unit             string             `json:"unit"`
	Value            map[string]float64 `json:"value"`
}

type result struct {
	ErrorMessage string `json:"errorMessage"`
	RequestID    string `json:"requestId"`
	Timestamp    int64  `json:"timestamp"`
}

type requestError struct {
	id  request.ResultID
	err error
}

// arn struct separate the Amazon Resource Name into individual fields.
type arn struct {
	Partition string
	Service   string
	Region    string
	AccountID string
	Resource  string
}

// Authenticator provides authentication and authorization support.
type Authenticator interface {
	Authenticate(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error)
}

// Handler returns a request.Handler for managing firehose requests.
func Handler(processor model.BatchProcessor, authenticator Authenticator) request.Handler {
	handle := func(c *request.Context) (*result, error) {
		accessKey := c.Request.Header.Get("X-Amz-Firehose-Access-Key")
		if accessKey == "" {
			return nil, requestError{
				id:  request.IDResponseErrorsUnauthorized,
				err: errors.New("Access key is required for using /firehose endpoint"),
			}
		}

		details, authorizer, err := authenticator.Authenticate(c.Request.Context(), headers.APIKey, accessKey)
		if err != nil {
			return nil, requestError{
				id:  request.IDResponseErrorsUnauthorized,
				err: errors.New("authentication failed"),
			}
		}

		c.Authentication = details
		c.Request = c.Request.WithContext(auth.ContextWithAuthorizer(c.Request.Context(), authorizer))
		if c.Request.Method != http.MethodPost {
			return nil, requestError{
				id:  request.IDResponseErrorsMethodNotAllowed,
				err: errors.New("only POST requests are supported"),
			}
		}

		var firehose firehose
		err = json.NewDecoder(c.Request.Body).Decode(&firehose)
		if err != nil {
			return nil, err
		}

		// convert firehose log to events
		baseEvent := requestMetadata(c)
		batch, err := processFirehose(firehose, baseEvent)
		if err != nil {
			return nil, err
		}

		if err := processor.ProcessBatch(c.Request.Context(), &batch); err != nil {
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
		// Set required requestId and timestamp to match Firehose HTTP delivery
		// request response format.
		// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
		return &result{RequestID: firehose.RequestID, Timestamp: firehose.Timestamp}, nil
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
			c.Result.StatusCode = 200
		}

		// Set response header
		c.Header().Set(headers.ContentType, "application/json")
		c.Write()
	}
}

func (e requestError) Error() string {
	return e.err.Error()
}

func processFirehose(firehose firehose, baseEvent model.APMEvent) (model.Batch, error) {
	var batch model.Batch
	for _, record := range firehose.Records {
		event := baseEvent
		recordDec, err := base64.StdEncoding.DecodeString(record.Data)
		if err != nil {
			return nil, err
		}

		splitLines := strings.Split(string(recordDec), "\n")
		for _, line := range splitLines {
			if line == "" {
				break
			}

			var cwMetric cloudwatchMetric
			err = json.Unmarshal([]byte(line), &cwMetric)
			if err == nil && cwMetric.MetricStreamName != "" {
				event = processMetrics(baseEvent, cwMetric)
			} else {
				event = processLogs(baseEvent, firehose, line)
			}
			batch = append(batch, event)
		}
	}
	return batch, nil
}

func processMetrics(event model.APMEvent, cwMetric cloudwatchMetric) model.APMEvent {
	event.Processor = model.MetricsetProcessor
	event.DataStream.Type = datastreams.MetricsType
	event.Timestamp = time.Unix(cwMetric.Timestamp/1000, 0)
	event.Cloud.AccountID = cwMetric.AccountID
	event.Cloud.Region = cwMetric.Region
	event.Cloud.ServiceName = cwMetric.Namespace

	namespace := strings.ToLower(cwMetric.Namespace)
	namespace = strings.ReplaceAll(namespace, "/", ".")
	event.DataStream.Dataset = namespace

	labels := common.MapStr{}
	for k, v := range cwMetric.Dimensions {
		labels[k] = v
	}
	event.Labels = labels

	var metricset model.Metricset
	metricset.Name = namespace

	samples := map[string]model.MetricsetSample{}
	var sample model.MetricsetSample
	var summary model.Summary
	for k, v := range cwMetric.Value {
		// TODO: handle units for CloudWatch metrics
		value := v
		switch k {
		case "min":
			summary.Min = &value
		case "max":
			summary.Max = &value
		case "sum":
			summary.Sum = &value
		case "count":
			vi := int64(value)
			summary.ValueCount = &vi
		}
	}

	sample.Type = model.MetricTypeSummary
	sample.Summary = summary
	samples[cwMetric.MetricName] = sample
	metricset.Samples = samples
	event.Metricset = &metricset
	return event
}

func processLogs(event model.APMEvent, firehose firehose, logLine string) model.APMEvent {
	event.DataStream.Dataset = dataset
	event.Processor = model.LogProcessor
	event.DataStream.Type = datastreams.LogsType
	event.Timestamp = time.Unix(firehose.Timestamp/1000, 0)
	event.Message = logLine
	return event
}

func requestMetadata(c *request.Context) model.APMEvent {
	arnString := c.Request.Header.Get("X-Amz-Firehose-Source-Arn")
	arnParsed := parseARN(arnString)

	var event model.APMEvent

	cloudOrigin := &model.CloudOrigin{}
	cloudOrigin.AccountID = arnParsed.AccountID
	cloudOrigin.Region = arnParsed.Region
	event.Cloud.Origin = cloudOrigin

	serviceOrigin := &model.ServiceOrigin{}
	serviceOrigin.ID = arnString
	serviceOrigin.Name = arnParsed.Resource
	event.Service.Origin = serviceOrigin
	return event
}

func parseARN(arnString string) arn {
	// arn example for firehose:
	// arn:aws:firehose:us-east-1:123456789:deliverystream/vpc-flow-log-stream-http-endpoint
	arnSections := 6
	sections := strings.SplitN(arnString, ":", arnSections)
	if len(sections) != arnSections {
		return arn{}
	}
	return arn{
		Partition: sections[1],
		Service:   sections[2],
		Region:    sections[3],
		AccountID: sections[4],
		Resource:  sections[5],
	}
}
