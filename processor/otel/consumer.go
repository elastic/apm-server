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

package otel

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
)

const (
	AgentNameJaeger = "Jaeger"

	keywordLength = 1024
	dot           = "."
	underscore    = "_"

	outcomeSuccess = "success"
	outcomeFailure = "failure"
	outcomeUnknown = "unknown"
)

// Consumer transforms open-telemetry data to be compatible with elastic APM data
type Consumer struct {
	Reporter publish.Reporter
}

// ConsumeTraces consumes OpenTelemetry trace data,
// converting into Elastic APM events and reporting to the Elastic APM schema.
func (c *Consumer) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	batch := c.convert(traces)
	return c.Reporter(ctx, publish.PendingReq{
		Transformables: batch.Transformables(),
		Trace:          true,
	})
}

func (c *Consumer) convert(td pdata.Traces) *model.Batch {
	batch := model.Batch{}
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		c.convertResourceSpans(resourceSpans.At(i), &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceSpans(resourceSpans pdata.ResourceSpans, out *model.Batch) {
	var metadata model.Metadata
	translateResourceMetadata(resourceSpans.Resource(), &metadata)
	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans()
	for i := 0; i < instrumentationLibrarySpans.Len(); i++ {
		c.convertInstrumentationLibrarySpans(instrumentationLibrarySpans.At(i), metadata, out)
	}
}

func (c *Consumer) convertInstrumentationLibrarySpans(in pdata.InstrumentationLibrarySpans, metadata model.Metadata, out *model.Batch) {
	otelSpans := in.Spans()
	for i := 0; i < otelSpans.Len(); i++ {
		c.convertSpan(otelSpans.At(i), metadata, out)
	}
}

func (c *Consumer) convertSpan(otelSpan pdata.Span, metadata model.Metadata, out *model.Batch) {
	logger := logp.NewLogger(logs.Otel)

	root := !otelSpan.ParentSpanID().IsValid()

	var parentID, spanID, traceID string
	if strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger") {
		if !root {
			parentID = formatJaegerSpanID(otelSpan.ParentSpanID())
		}
		traceID = formatJaegerTraceID(otelSpan.TraceID())
		spanID = formatJaegerSpanID(otelSpan.SpanID())
	} else {
		if !root {
			parentID = otelSpan.ParentSpanID().HexString()
		}
		traceID = otelSpan.TraceID().HexString()
		spanID = otelSpan.SpanID().HexString()
	}

	startTime := pdata.UnixNanoToTime(otelSpan.StartTime())
	endTime := pdata.UnixNanoToTime(otelSpan.EndTime())
	var durationMillis float64
	if endTime.After(startTime) {
		durationMillis = endTime.Sub(startTime).Seconds() * 1000
	}

	var transaction *model.Transaction
	var span *model.Span

	name := otelSpan.Name()
	if root || otelSpan.Kind() == pdata.SpanKindSERVER {
		transaction = &model.Transaction{
			Metadata:  metadata,
			ID:        spanID,
			ParentID:  parentID,
			TraceID:   traceID,
			Timestamp: startTime,
			Duration:  durationMillis,
			Name:      name,
		}
		translateTransaction(otelSpan, metadata, transaction)
		out.Transactions = append(out.Transactions, transaction)
	} else {
		span = &model.Span{
			Metadata:  metadata,
			ID:        spanID,
			ParentID:  parentID,
			TraceID:   traceID,
			Timestamp: startTime,
			Duration:  durationMillis,
			Name:      name,
			Outcome:   outcomeUnknown,
		}
		translateSpan(otelSpan, metadata, span)
		out.Spans = append(out.Spans, span)
	}

	events := otelSpan.Events()
	for i := 0; i < events.Len(); i++ {
		convertSpanEvent(logger, events.At(i), metadata, transaction, span, out)
	}
}

func translateTransaction(span pdata.Span, metadata model.Metadata, event *model.Transaction) {
	isJaeger := strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger")
	labels := make(common.MapStr)
	var http model.Http
	var httpStatusCode int
	var message model.Message
	var component string
	var outcome, result string
	var isHTTP, isMessaging bool
	var samplerType, samplerParam pdata.AttributeValue
	span.Attributes().ForEach(func(kDots string, v pdata.AttributeValue) {
		if isJaeger {
			switch kDots {
			case "sampler.type":
				samplerType = v
				return
			case "sampler.param":
				samplerParam = v
				return
			}
		}

		k := replaceDots(kDots)
		switch v.Type() {
		case pdata.AttributeValueBOOL:
			labels[k] = v.BoolVal()
		case pdata.AttributeValueDOUBLE:
			labels[k] = v.DoubleVal()
		case pdata.AttributeValueINT:
			switch kDots {
			case "http.status_code":
				httpStatusCode = int(v.IntVal())
				isHTTP = true
			default:
				labels[k] = v.IntVal()
			}
		case pdata.AttributeValueSTRING:
			stringval := truncate(v.StringVal())
			switch kDots {
			case "span.kind": // filter out
			case "http.method":
				http.Request = &model.Req{Method: stringval}
				isHTTP = true
			case "http.url", "http.path":
				event.URL = model.ParseURL(stringval, metadata.System.DetectedHostname)
				isHTTP = true
			case "http.status_code":
				if intv, err := strconv.Atoi(stringval); err == nil {
					httpStatusCode = intv
				}
				isHTTP = true
			case "http.protocol":
				if strings.HasPrefix(stringval, "HTTP/") {
					version := strings.TrimPrefix(stringval, "HTTP/")
					http.Version = &version
				} else {
					labels[k] = stringval
				}
				isHTTP = true
			case "message_bus.destination":
				message.QueueName = &stringval
				isMessaging = true
			case "type":
				event.Type = stringval
			case "service.version":
				event.Metadata.Service.Version = stringval
			case "component":
				component = stringval
				fallthrough
			default:
				labels[k] = stringval
			}
		}
	})

	if event.Type == "" {
		if isHTTP {
			event.Type = "request"
		} else if isMessaging {
			event.Type = "messaging"
		} else if component != "" {
			event.Type = component
		} else {
			event.Type = "custom"
		}
	}

	if isHTTP {
		if httpStatusCode > 0 {
			http.Response = &model.Resp{MinimalResp: model.MinimalResp{StatusCode: &httpStatusCode}}
			result = statusCodeResult(httpStatusCode)
			outcome = serverStatusCodeOutcome(httpStatusCode)
		}
		event.HTTP = &http
	} else if isMessaging {
		event.Message = &message
	}

	if result == "" {
		if span.Status().Code() == pdata.StatusCodeError {
			result = "Error"
			outcome = outcomeFailure
		} else {
			result = "Success"
			outcome = outcomeSuccess
		}
	}
	event.Result = result
	event.Outcome = outcome

	if samplerType != (pdata.AttributeValue{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate span metrics.
		parseSamplerAttributes(samplerType, samplerParam, &event.RepresentativeCount, labels)
	}

	event.Labels = labels
}

func translateSpan(span pdata.Span, metadata model.Metadata, event *model.Span) {
	isJaeger := strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger")
	labels := make(common.MapStr)

	var http model.HTTP
	var message model.Message
	var db model.DB
	var destination model.Destination
	var destinationService model.DestinationService
	var isDBSpan, isHTTPSpan, isMessagingSpan bool
	var component string
	var samplerType, samplerParam pdata.AttributeValue
	span.Attributes().ForEach(func(kDots string, v pdata.AttributeValue) {
		if isJaeger {
			switch kDots {
			case "sampler.type":
				samplerType = v
				return
			case "sampler.param":
				samplerParam = v
				return
			}
		}

		k := replaceDots(kDots)
		switch v.Type() {
		case pdata.AttributeValueBOOL:
			labels[k] = v.BoolVal()
		case pdata.AttributeValueDOUBLE:
			labels[k] = v.DoubleVal()
		case pdata.AttributeValueINT:
			switch kDots {
			case "http.status_code":
				code := int(v.IntVal())
				http.StatusCode = &code
				isHTTPSpan = true
			case "peer.port":
				port := int(v.IntVal())
				destination.Port = &port
			default:
				labels[k] = v.IntVal()
			}
		case pdata.AttributeValueSTRING:
			stringval := truncate(v.StringVal())
			switch kDots {
			case "span.kind": // filter out
			case "http.url":
				http.URL = &stringval
				isHTTPSpan = true
			case "http.method":
				http.Method = &stringval
				isHTTPSpan = true
			case "sql.query":
				db.Statement = &stringval
				if db.Type == nil {
					dbType := "sql"
					db.Type = &dbType
				}
				isDBSpan = true
			case "db.statement":
				db.Statement = &stringval
				isDBSpan = true
			case "db.instance":
				db.Instance = &stringval
				isDBSpan = true
			case "db.type":
				db.Type = &stringval
				isDBSpan = true
			case "db.user":
				db.UserName = &stringval
				isDBSpan = true
			case "peer.address":
				destinationService.Resource = &stringval
				if !strings.ContainsRune(stringval, ':') || net.ParseIP(stringval) != nil {
					// peer.address is not necessarily a hostname
					// or IP address; it could be something like
					// a JDBC connection string or ip:port. Ignore
					// values containing colons, except for IPv6.
					destination.Address = &stringval
				}
			case "peer.hostname", "peer.ipv4", "peer.ipv6":
				destination.Address = &stringval
			case "peer.service":
				destinationService.Name = &stringval
				if destinationService.Resource == nil {
					// Prefer using peer.address for resource.
					destinationService.Resource = &stringval
				}
			case "message_bus.destination":
				message.QueueName = &stringval
				isMessagingSpan = true
			case "component":
				component = stringval
				fallthrough
			default:
				labels[k] = stringval
			}
		}
	})

	if http.URL != nil {
		if fullURL, err := url.Parse(*http.URL); err == nil {
			url := url.URL{Scheme: fullURL.Scheme, Host: fullURL.Host}
			hostname := truncate(url.Hostname())
			var port int
			portString := url.Port()
			if portString != "" {
				port, _ = strconv.Atoi(portString)
			} else {
				port = schemeDefaultPort(url.Scheme)
			}

			// Set destination.{address,port} from the HTTP URL,
			// replacing peer.* based values to ensure consistency.
			destination = model.Destination{Address: &hostname}
			if port > 0 {
				destination.Port = &port
			}

			// Set destination.service.* from the HTTP URL,
			// unless peer.service was specified.
			if destinationService.Name == nil {
				resource := url.Host
				if port > 0 && port == schemeDefaultPort(url.Scheme) {
					hasDefaultPort := portString != ""
					if hasDefaultPort {
						// Remove the default port from destination.service.name.
						url.Host = hostname
					} else {
						// Add the default port to destination.service.resource.
						resource = fmt.Sprintf("%s:%d", resource, port)
					}
				}
				name := url.String()
				destinationService.Name = &name
				destinationService.Resource = &resource
			}
		}
	}

	if destination != (model.Destination{}) {
		event.Destination = &destination
	}

	switch {
	case isHTTPSpan:
		if http.StatusCode != nil {
			event.Outcome = clientStatusCodeOutcome(*http.StatusCode)
		}
		event.Type = "external"
		subtype := "http"
		event.Subtype = &subtype
		event.HTTP = &http
	case isDBSpan:
		event.Type = "db"
		if db.Type != nil && *db.Type != "" {
			event.Subtype = db.Type
		}
		event.DB = &db
	case isMessagingSpan:
		event.Type = "messaging"
		event.Message = &message
	default:
		event.Type = "custom"
		if component != "" {
			event.Subtype = &component
		}
	}

	if destinationService != (model.DestinationService{}) {
		if destinationService.Type == nil {
			// Copy span type to destination.service.type.
			destinationService.Type = &event.Type
		}
		event.DestinationService = &destinationService
	}

	if samplerType != (pdata.AttributeValue{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate transaction metrics.
		parseSamplerAttributes(samplerType, samplerParam, &event.RepresentativeCount, labels)
	}

	event.Labels = labels
}

func parseSamplerAttributes(samplerType, samplerParam pdata.AttributeValue, representativeCount *float64, labels common.MapStr) {
	switch samplerType := samplerType.StringVal(); samplerType {
	case "probabilistic":
		probability := samplerParam.DoubleVal()
		if probability > 0 && probability <= 1 {
			*representativeCount = 1 / probability
		}
	default:
		labels["sampler_type"] = samplerType
		switch samplerParam.Type() {
		case pdata.AttributeValueBOOL:
			labels["sampler_param"] = samplerParam.BoolVal()
		case pdata.AttributeValueDOUBLE:
			labels["sampler_param"] = samplerParam.DoubleVal()
		}
	}
}

func convertSpanEvent(
	logger *logp.Logger,
	event pdata.SpanEvent,
	metadata model.Metadata,
	transaction *model.Transaction, span *model.Span, // only one is non-nil
	out *model.Batch,
) {
	var isError bool
	var exMessage, exType string
	logMessage := event.Name()
	hasMinimalInfo := logMessage != ""
	isJaeger := strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger")
	event.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		if !isJaeger {
			return
		}
		if v.Type() != pdata.AttributeValueSTRING {
			return
		}
		stringval := truncate(v.StringVal())
		switch k {
		case "error", "error.object":
			exMessage = stringval
			hasMinimalInfo = true
			isError = true
		case "event":
			if stringval == "error" { // according to opentracing spec
				isError = true
			} else if logMessage == "" {
				// Jaeger seems to send the message in the 'event' field.
				//
				// In case 'message' is sent, the event's name will be set
				// and we will use that. Otherwise we use 'event'.
				logMessage = stringval
				hasMinimalInfo = true
			}
		case "error.kind":
			exType = stringval
			hasMinimalInfo = true
			isError = true
		case "level":
			isError = stringval == "error"
		}
	})
	if !isError {
		return
	}
	if !hasMinimalInfo {
		logger.Debugf("Cannot convert span event (name=%q) into elastic apm error: %v", event.Name())
		return
	}

	e := &model.Error{
		Timestamp: pdata.UnixNanoToTime(event.Timestamp()),
	}
	if logMessage != "" {
		e.Log = &model.Log{Message: logMessage}
	}
	if exMessage != "" || exType != "" {
		e.Exception = &model.Exception{}
		if exMessage != "" {
			e.Exception.Message = &exMessage
		}
		if exType != "" {
			e.Exception.Type = &exType
		}
	}
	if transaction != nil {
		addTransactionCtxToErr(transaction, e)
	}
	if span != nil {
		addSpanCtxToErr(span, metadata.System.DetectedHostname, e)
	}
	out.Errors = append(out.Errors, e)
}

func addTransactionCtxToErr(transaction *model.Transaction, err *model.Error) {
	err.Metadata = transaction.Metadata
	err.TransactionID = transaction.ID
	err.TraceID = transaction.TraceID
	err.ParentID = transaction.ID
	err.HTTP = transaction.HTTP
	err.URL = transaction.URL
	err.TransactionType = &transaction.Type
}

func addSpanCtxToErr(span *model.Span, hostname string, err *model.Error) {
	err.Metadata = span.Metadata
	err.TransactionID = span.TransactionID
	err.TraceID = span.TraceID
	err.ParentID = span.ID
	if span.HTTP != nil {
		err.HTTP = &model.Http{}
		if span.HTTP.StatusCode != nil {
			err.HTTP.Response = &model.Resp{MinimalResp: model.MinimalResp{StatusCode: span.HTTP.StatusCode}}
		}
		if span.HTTP.Method != nil {
			err.HTTP.Request = &model.Req{Method: *span.HTTP.Method}
		}
		if span.HTTP.URL != nil {
			err.URL = model.ParseURL(*span.HTTP.URL, hostname)
		}
	}
}

func replaceDots(s string) string {
	return strings.ReplaceAll(s, dot, underscore)
}

// copied from elastic go-apm agent

var standardStatusCodeResults = [...]string{
	"HTTP 1xx",
	"HTTP 2xx",
	"HTTP 3xx",
	"HTTP 4xx",
	"HTTP 5xx",
}

// statusCodeResult returns the transaction result value to use for the given status code.
func statusCodeResult(statusCode int) string {
	switch i := statusCode / 100; i {
	case 1, 2, 3, 4, 5:
		return standardStatusCodeResults[i-1]
	}
	return fmt.Sprintf("HTTP %d", statusCode)
}

// serverStatusCodeOutcome returns the transaction outcome value to use for the given status code.
func serverStatusCodeOutcome(statusCode int) string {
	if statusCode >= 500 {
		return outcomeFailure
	}
	return outcomeSuccess
}

// clientStatusCodeOutcome returns the span outcome value to use for the given status code.
func clientStatusCodeOutcome(statusCode int) string {
	if statusCode >= 400 {
		return outcomeFailure
	}
	return outcomeSuccess
}

// truncate returns s truncated at n runes, and the number of runes in the resulting string (<= n).
func truncate(s string) string {
	var j int
	for i := range s {
		if j == keywordLength {
			return s[:i]
		}
		j++
	}
	return s
}

// formatJaegerTraceID returns the traceID as string in Jaeger format (hexadecimal without leading zeros)
func formatJaegerTraceID(traceID pdata.TraceID) string {
	if !traceID.IsValid() {
		return ""
	}
	bytes := traceID.Bytes()
	jaegerTraceIDHigh := binary.BigEndian.Uint64(bytes[:8])
	jaegerTraceIDLow := binary.BigEndian.Uint64(bytes[8:])
	if jaegerTraceIDHigh == 0 {
		return fmt.Sprintf("%x", jaegerTraceIDLow)
	}
	return fmt.Sprintf("%x%016x", jaegerTraceIDHigh, jaegerTraceIDLow)
}

// formatJaegerSpanID returns the spanID as string in Jaeger format (hexadecimal without leading zeros)
func formatJaegerSpanID(spanID pdata.SpanID) string {
	if !spanID.IsValid() {
		return ""
	}
	bytes := spanID.Bytes()
	jaegerSpanID := binary.BigEndian.Uint64(bytes[:])
	return fmt.Sprintf("%x", jaegerSpanID)
}

func schemeDefaultPort(scheme string) int {
	switch scheme {
	case "http":
		return 80
	case "https":
		return 443
	}
	return 0
}
