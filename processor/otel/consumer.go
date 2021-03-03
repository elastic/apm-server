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

// Portions copied from OpenTelemetry Collector (contrib), from the
// elastic exporter.
//
// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
)

const (
	keywordLength = 1024
	dot           = "."
	underscore    = "_"

	outcomeSuccess = "success"
	outcomeFailure = "failure"
	outcomeUnknown = "unknown"
)

// Consumer transforms open-telemetry data to be compatible with elastic APM data
type Consumer struct {
	stats consumerStats

	Processor model.BatchProcessor
}

// ConsumerStats holds a snapshot of statistics about data consumption.
type ConsumerStats struct {
	// UnsupportedMetricsDropped records the number of unsupported metrics
	// that have been dropped by the consumer.
	UnsupportedMetricsDropped int64
}

// consumerStats holds the current statistics, which must be accessed and
// modified using atomic operations.
type consumerStats struct {
	unsupportedMetricsDropped int64
}

// Stats returns a snapshot of the current statistics about data consumption.
func (c *Consumer) Stats() ConsumerStats {
	return ConsumerStats{
		UnsupportedMetricsDropped: atomic.LoadInt64(&c.stats.unsupportedMetricsDropped),
	}
}

// ConsumeTraces consumes OpenTelemetry trace data,
// converting into Elastic APM events and reporting to the Elastic APM schema.
func (c *Consumer) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	batch := c.convert(traces)
	return c.Processor.ProcessBatch(ctx, batch)
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
		c.convertSpan(otelSpans.At(i), in.InstrumentationLibrary(), metadata, out)
	}
}

func (c *Consumer) convertSpan(
	otelSpan pdata.Span,
	otelLibrary pdata.InstrumentationLibrary,
	metadata model.Metadata,
	out *model.Batch,
) {
	logger := logp.NewLogger(logs.Otel)

	root := !otelSpan.ParentSpanID().IsValid()

	var parentID string
	if !root {
		parentID = otelSpan.ParentSpanID().HexString()
	}

	traceID := otelSpan.TraceID().HexString()
	spanID := otelSpan.SpanID().HexString()

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
			Outcome:   spanStatusOutcome(otelSpan.Status()),
		}
		translateTransaction(otelSpan, otelLibrary, metadata, &transactionBuilder{Transaction: transaction})
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
			Outcome:   spanStatusOutcome(otelSpan.Status()),
		}
		translateSpan(otelSpan, metadata, span)
		out.Spans = append(out.Spans, span)
	}

	events := otelSpan.Events()
	for i := 0; i < events.Len(); i++ {
		convertSpanEvent(logger, events.At(i), metadata, transaction, span, out)
	}
}

func translateTransaction(
	span pdata.Span,
	library pdata.InstrumentationLibrary,
	metadata model.Metadata,
	tx *transactionBuilder,
) {
	isJaeger := strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger")
	labels := make(common.MapStr)

	var (
		netHostName string
		netHostPort int
		netPeerIP   string
		netPeerPort int
	)

	var message model.Message
	var component string
	var isMessaging bool
	var samplerType, samplerParam pdata.AttributeValue
	var httpHostName string
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
			case conventions.AttributeHTTPStatusCode:
				tx.setHTTPStatusCode(int(v.IntVal()))
			case conventions.AttributeNetPeerPort:
				netPeerPort = int(v.IntVal())
			case conventions.AttributeNetHostPort:
				netHostPort = int(v.IntVal())
			default:
				labels[k] = v.IntVal()
			}
		case pdata.AttributeValueSTRING:
			stringval := truncate(v.StringVal())
			switch kDots {
			// http.*
			case conventions.AttributeHTTPMethod:
				tx.setHTTPMethod(stringval)
			case conventions.AttributeHTTPURL, conventions.AttributeHTTPTarget, "http.path":
				tx.setHTTPURL(stringval)
			case conventions.AttributeHTTPHost:
				tx.setHTTPHost(stringval)
			case conventions.AttributeHTTPScheme:
				tx.setHTTPScheme(stringval)
			case conventions.AttributeHTTPStatusCode:
				if intv, err := strconv.Atoi(stringval); err == nil {
					tx.setHTTPStatusCode(intv)
				}
			case "http.protocol":
				if !strings.HasPrefix(stringval, "HTTP/") {
					// Unexpected, store in labels for debugging.
					labels[k] = stringval
					break
				}
				stringval = strings.TrimPrefix(stringval, "HTTP/")
				fallthrough
			case conventions.AttributeHTTPFlavor:
				tx.setHTTPVersion(stringval)
			case conventions.AttributeHTTPServerName:
				httpHostName = stringval
			case conventions.AttributeHTTPClientIP:
				tx.Metadata.Client.IP = net.ParseIP(stringval)
			case conventions.AttributeHTTPUserAgent:
				tx.Metadata.UserAgent.Original = stringval
			case "http.remote_addr":
				// NOTE(axw) this is non-standard, sent by opentelemetry-go's othttp.
				// It's semanticall equivalent to net.peer.ip+port. Standard attributes
				// take precedence.
				ip, port, err := net.SplitHostPort(stringval)
				if err != nil {
					ip = stringval
				}
				if net.ParseIP(ip) != nil {
					if netPeerIP == "" {
						netPeerIP = ip
					}
					if netPeerPort == 0 {
						netPeerPort, _ = strconv.Atoi(port)
					}
				}

			// net.*
			case conventions.AttributeNetPeerIP:
				netPeerIP = stringval
			case conventions.AttributeNetHostName:
				netHostName = stringval

			// messaging
			//
			// TODO(axw) translate OpenTelemtry messaging conventions.
			case "message_bus.destination":
				message.QueueName = stringval
				isMessaging = true

			// miscellaneous
			case "span.kind": // filter out
			case "type":
				tx.Type = stringval
			case conventions.AttributeServiceVersion:
				tx.Metadata.Service.Version = stringval
			case conventions.AttributeComponent:
				component = stringval
				fallthrough
			default:
				labels[k] = stringval
			}
		}
	})

	if tx.Type == "" {
		if tx.HTTP != nil {
			tx.Type = "request"
		} else if isMessaging {
			tx.Type = "messaging"
		} else if component != "" {
			tx.Type = component
		} else {
			tx.Type = "custom"
		}
	}

	if tx.HTTP != nil {
		// Build the model.URL from tx.http{URL,Host,Scheme}.
		httpHost := tx.httpHost
		if httpHost == "" {
			httpHost = httpHostName
			if httpHost == "" {
				httpHost = netHostName
				if httpHost == "" {
					httpHost = metadata.System.DetectedHostname
				}
			}
			if httpHost != "" && netHostPort > 0 {
				httpHost = net.JoinHostPort(httpHost, strconv.Itoa(netHostPort))
			}
		}
		tx.URL = model.ParseURL(tx.httpURL, httpHost, tx.httpScheme)

		// Set the remote address from net.peer.*
		if tx.HTTP.Request != nil && netPeerIP != "" {
			remoteAddr := netPeerIP
			if netPeerPort > 0 {
				remoteAddr = net.JoinHostPort(remoteAddr, strconv.Itoa(netPeerPort))
			}
			tx.setHTTPRemoteAddr(remoteAddr)
		}
	}

	if isMessaging {
		tx.Message = &message
	}

	if samplerType != (pdata.AttributeValue{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate span metrics.
		parseSamplerAttributes(samplerType, samplerParam, &tx.RepresentativeCount, labels)
	}

	if tx.Result == "" {
		tx.Result = spanStatusResult(span.Status())
	}
	tx.setFramework(library.Name(), library.Version())
	tx.Labels = labels
}

func translateSpan(span pdata.Span, metadata model.Metadata, event *model.Span) {
	isJaeger := strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger")
	labels := make(common.MapStr)

	var (
		netPeerName string
		netPeerIP   string
		netPeerPort int
	)

	var (
		httpURL    string
		httpHost   string
		httpTarget string
		httpScheme string = "http"
	)

	var http model.HTTP
	var message model.Message
	var db model.DB
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
			case conventions.AttributeNetPeerPort, "peer.port":
				netPeerPort = int(v.IntVal())
			default:
				labels[k] = v.IntVal()
			}
		case pdata.AttributeValueSTRING:
			stringval := truncate(v.StringVal())
			switch kDots {
			// http.*
			case conventions.AttributeHTTPHost:
				httpHost = stringval
				isHTTPSpan = true
			case conventions.AttributeHTTPScheme:
				httpScheme = stringval
				isHTTPSpan = true
			case conventions.AttributeHTTPTarget:
				httpTarget = stringval
				isHTTPSpan = true
			case conventions.AttributeHTTPURL:
				httpURL = stringval
				isHTTPSpan = true
			case conventions.AttributeHTTPMethod:
				http.Method = stringval
				isHTTPSpan = true

			// db.*
			case "sql.query":
				if db.Type == "" {
					db.Type = "sql"
				}
				fallthrough
			case conventions.AttributeDBStatement:
				db.Statement = stringval
				isDBSpan = true
			case conventions.AttributeDBName, "db.instance":
				db.Instance = stringval
				isDBSpan = true
			case conventions.AttributeDBSystem, "db.type":
				db.Type = stringval
				isDBSpan = true
			case conventions.AttributeDBUser:
				db.UserName = stringval
				isDBSpan = true

			// net.*
			case conventions.AttributeNetPeerName, "peer.hostname":
				netPeerName = stringval
			case conventions.AttributeNetPeerIP, "peer.ipv4", "peer.ipv6":
				netPeerIP = stringval
			case "peer.address":
				destinationService.Resource = stringval
				if !strings.ContainsRune(stringval, ':') || net.ParseIP(stringval) != nil {
					// peer.address is not necessarily a hostname
					// or IP address; it could be something like
					// a JDBC connection string or ip:port. Ignore
					// values containing colons, except for IPv6.
					netPeerName = stringval
				}

			// messaging
			//
			// TODO(axw) translate OpenTelemtry messaging conventions.
			case "message_bus.destination":
				message.QueueName = stringval
				isMessagingSpan = true

			// miscellaneous
			case "span.kind": // filter out
			case conventions.AttributePeerService:
				destinationService.Name = stringval
				if destinationService.Resource == "" {
					// Prefer using peer.address for resource.
					destinationService.Resource = stringval
				}
			case conventions.AttributeComponent:
				component = stringval
				fallthrough
			default:
				labels[k] = stringval
			}
		}
	})

	destPort := netPeerPort
	destAddr := netPeerName
	if destAddr == "" {
		destAddr = netPeerIP
	}

	if isHTTPSpan {
		var fullURL *url.URL
		if httpURL != "" {
			fullURL, _ = url.Parse(httpURL)
		} else if httpTarget != "" {
			// Build http.url from http.scheme, http.target, etc.
			if u, err := url.Parse(httpTarget); err == nil {
				fullURL = u
				fullURL.Scheme = httpScheme
				if httpHost == "" {
					// Set host from net.peer.*
					httpHost = destAddr
					if netPeerPort > 0 {
						httpHost = net.JoinHostPort(httpHost, strconv.Itoa(destPort))
					}
				}
				fullURL.Host = httpHost
				httpURL = fullURL.String()
			}
		}
		if fullURL != nil {
			http.URL = httpURL
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
			destAddr = hostname
			if port > 0 {
				destPort = port
			}

			// Set destination.service.* from the HTTP URL, unless peer.service was specified.
			if destinationService.Name == "" {
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
				destinationService.Name = url.String()
				destinationService.Resource = resource
			}
		}
	}

	switch {
	case isHTTPSpan:
		if http.StatusCode != nil {
			if event.Outcome == outcomeUnknown {
				event.Outcome = clientHTTPStatusCodeOutcome(*http.StatusCode)
			}
		}
		event.Type = "external"
		subtype := "http"
		event.Subtype = subtype
		event.HTTP = &http
	case isDBSpan:
		event.Type = "db"
		if db.Type != "" {
			event.Subtype = db.Type
			if destinationService.Name == "" {
				// For database requests, we currently just identify
				// the destination service by db.system.
				destinationService.Name = event.Subtype
				destinationService.Resource = event.Subtype
			}
		}
		event.DB = &db
	case isMessagingSpan:
		event.Type = "messaging"
		event.Message = &message
	default:
		event.Type = "app"
		event.Subtype = component
	}

	if destAddr != "" {
		event.Destination = &model.Destination{Address: destAddr}
		if destPort > 0 {
			event.Destination.Port = &destPort
		}
	}
	if destinationService != (model.DestinationService{}) {
		if destinationService.Type == "" {
			// Copy span type to destination.service.type.
			destinationService.Type = event.Type
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
			// TODO(axw) translate OpenTelemetry exception span events.
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
		e.Exception = &model.Exception{
			Message: exMessage,
			Type:    exType,
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
	err.TransactionType = transaction.Type
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
		if span.HTTP.Method != "" {
			err.HTTP.Request = &model.Req{Method: span.HTTP.Method}
		}
		if span.HTTP.URL != "" {
			err.URL = model.ParseURL(span.HTTP.URL, hostname, "")
		}
	}
}

func replaceDots(s string) string {
	return strings.ReplaceAll(s, dot, underscore)
}

// spanStatusOutcome returns the outcome for transactions and spans based on
// the given OTLP span status.
func spanStatusOutcome(status pdata.SpanStatus) string {
	switch status.Code() {
	case pdata.StatusCodeOk:
		return outcomeSuccess
	case pdata.StatusCodeError:
		return outcomeFailure
	}
	return outcomeUnknown
}

// spanStatusResult returns the result for transactions based on the given
// OTLP span status. If the span status is unknown, an empty result string
// is returned.
func spanStatusResult(status pdata.SpanStatus) string {
	switch status.Code() {
	case pdata.StatusCodeOk:
		return "Success"
	case pdata.StatusCodeError:
		return "Error"
	}
	return ""
}

// clientHTTPStatusCodeOutcome returns the span outcome value to use for the
// given HTTP status code.
func clientHTTPStatusCodeOutcome(statusCode int) string {
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

func schemeDefaultPort(scheme string) int {
	switch scheme {
	case "http":
		return 80
	case "https":
		return 443
	}
	return 0
}
