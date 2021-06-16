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
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"google.golang.org/grpc/codes"

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

// Capabilities is part of the consumer interfaces.
func (c *Consumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

// ConsumeTraces consumes OpenTelemetry trace data,
// converting into Elastic APM events and reporting to the Elastic APM schema.
func (c *Consumer) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	receiveTimestamp := time.Now()
	batch := c.convert(traces, receiveTimestamp)
	return c.Processor.ProcessBatch(ctx, batch)
}

func (c *Consumer) convert(td pdata.Traces, receiveTimestamp time.Time) *model.Batch {
	batch := model.Batch{}
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		c.convertResourceSpans(resourceSpans.At(i), receiveTimestamp, &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceSpans(resourceSpans pdata.ResourceSpans, receiveTimestamp time.Time, out *model.Batch) {
	var metadata model.Metadata
	var timeDelta time.Duration
	resource := resourceSpans.Resource()
	translateResourceMetadata(resource, &metadata)
	if exportTimestamp, ok := exportTimestamp(resource); ok {
		timeDelta = receiveTimestamp.Sub(exportTimestamp)
	}
	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans()
	for i := 0; i < instrumentationLibrarySpans.Len(); i++ {
		c.convertInstrumentationLibrarySpans(instrumentationLibrarySpans.At(i), metadata, timeDelta, out)
	}
}

func (c *Consumer) convertInstrumentationLibrarySpans(
	in pdata.InstrumentationLibrarySpans,
	metadata model.Metadata,
	timeDelta time.Duration,
	out *model.Batch,
) {
	otelSpans := in.Spans()
	for i := 0; i < otelSpans.Len(); i++ {
		c.convertSpan(otelSpans.At(i), in.InstrumentationLibrary(), metadata, timeDelta, out)
	}
}

func (c *Consumer) convertSpan(
	otelSpan pdata.Span,
	otelLibrary pdata.InstrumentationLibrary,
	metadata model.Metadata,
	timeDelta time.Duration,
	out *model.Batch,
) {
	logger := logp.NewLogger(logs.Otel)

	root := otelSpan.ParentSpanID().IsEmpty()
	var parentID string
	if !root {
		parentID = otelSpan.ParentSpanID().HexString()
	}

	traceID := otelSpan.TraceID().HexString()
	spanID := otelSpan.SpanID().HexString()

	startTime := otelSpan.StartTimestamp().AsTime()
	endTime := otelSpan.EndTimestamp().AsTime()
	var durationMillis float64
	if endTime.After(startTime) {
		durationMillis = endTime.Sub(startTime).Seconds() * 1000
	}
	timestamp := startTime.Add(timeDelta)

	var transaction *model.Transaction
	var span *model.Span

	name := otelSpan.Name()
	// Message consumption results in either a transaction or a span based
	// on whether the consumption is active or passive. Otel spans
	// currently do not have the metadata to make this distinction. For
	// now, we assume that the majority of consumption is passive, and
	// therefore start a transaction whenever span kind == consumer.
	if root || otelSpan.Kind() == pdata.SpanKindServer || otelSpan.Kind() == pdata.SpanKindConsumer {
		transaction = &model.Transaction{
			Metadata:  metadata,
			ID:        spanID,
			ParentID:  parentID,
			TraceID:   traceID,
			Timestamp: timestamp,
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
			Timestamp: timestamp,
			Duration:  durationMillis,
			Name:      name,
			Outcome:   spanStatusOutcome(otelSpan.Status()),
		}
		translateSpan(otelSpan, metadata, span)
		out.Spans = append(out.Spans, span)
	}

	events := otelSpan.Events()
	for i := 0; i < events.Len(); i++ {
		convertSpanEvent(logger, events.At(i), metadata, transaction, span, timeDelta, out)
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
		netPeerName string
		netPeerPort int
	)

	var message model.Message
	var component string
	var isMessaging bool
	var samplerType, samplerParam pdata.AttributeValue
	var httpHostName string
	span.Attributes().Range(func(kDots string, v pdata.AttributeValue) bool {
		if isJaeger {
			switch kDots {
			case "sampler.type":
				samplerType = v
				return true
			case "sampler.param":
				samplerParam = v
				return true
			}
		}

		k := replaceDots(kDots)
		switch v.Type() {
		case pdata.AttributeValueTypeArray:
			array := v.ArrayVal()
			values := make([]interface{}, array.Len())
			for i := range values {
				value := array.At(i)
				switch value.Type() {
				case pdata.AttributeValueTypeBool:
					values[i] = value.BoolVal()
				case pdata.AttributeValueTypeDouble:
					values[i] = value.DoubleVal()
				case pdata.AttributeValueTypeInt:
					values[i] = value.IntVal()
				case pdata.AttributeValueTypeString:
					values[i] = truncate(value.StringVal())
				}
			}
			labels[k] = values
		case pdata.AttributeValueTypeBool:
			labels[k] = v.BoolVal()
		case pdata.AttributeValueTypeDouble:
			labels[k] = v.DoubleVal()
		case pdata.AttributeValueTypeInt:
			switch kDots {
			case conventions.AttributeHTTPStatusCode:
				tx.setHTTPStatusCode(int(v.IntVal()))
			case conventions.AttributeNetPeerPort:
				netPeerPort = int(v.IntVal())
			case conventions.AttributeNetHostPort:
				netHostPort = int(v.IntVal())
			case "rpc.grpc.status_code":
				tx.Result = codes.Code(v.IntVal()).String()
			default:
				labels[k] = v.IntVal()
			}
		case pdata.AttributeValueTypeString:
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
			case conventions.AttributeNetPeerName:
				netPeerName = stringval
			case conventions.AttributeNetHostName:
				netHostName = stringval

			// messaging.*
			case "message_bus.destination", conventions.AttributeMessagingDestination:
				message.QueueName = stringval
				isMessaging = true

			// rpc.*
			//
			// TODO(axw) add RPC fieldset to ECS? Currently we drop these
			// attributes, and rely on the operation name like we do with
			// Elastic APM agents.
			case conventions.AttributeRPCSystem:
				tx.Type = "request"
			case conventions.AttributeRPCService:
			case conventions.AttributeRPCMethod:

			// miscellaneous
			case "span.kind": // filter out
			case "type":
				tx.Type = stringval
			case conventions.AttributeServiceVersion:
				tx.Metadata.Service.Version = stringval
			case "component":
				component = stringval
				fallthrough
			default:
				labels[k] = stringval
			}
		}
		return true
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

	if netPeerIP != "" {
		tx.Metadata.Client.IP = net.ParseIP(netPeerIP)
	}
	tx.Metadata.Client.Port = netPeerPort
	tx.Metadata.Client.Domain = netPeerName

	if samplerType != (pdata.AttributeValue{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate span metrics.
		parseSamplerAttributes(samplerType, samplerParam, &tx.RepresentativeCount, labels)
	} else {
		tx.RepresentativeCount = 1
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

	var (
		messageSystem    string
		messageOperation string
	)

	var http model.HTTP
	var message model.Message
	var db model.DB
	var destinationService model.DestinationService
	var isDBSpan, isHTTPSpan, isMessagingSpan, isRPCSpan bool
	var component string
	var rpcSystem string
	var samplerType, samplerParam pdata.AttributeValue
	span.Attributes().Range(func(kDots string, v pdata.AttributeValue) bool {
		if isJaeger {
			switch kDots {
			case "sampler.type":
				samplerType = v
				return true
			case "sampler.param":
				samplerParam = v
				return true
			}
		}

		k := replaceDots(kDots)
		switch v.Type() {
		case pdata.AttributeValueTypeBool:
			labels[k] = v.BoolVal()
		case pdata.AttributeValueTypeDouble:
			labels[k] = v.DoubleVal()
		case pdata.AttributeValueTypeInt:
			switch kDots {
			case "http.status_code":
				http.StatusCode = int(v.IntVal())
				isHTTPSpan = true
			case conventions.AttributeNetPeerPort, "peer.port":
				netPeerPort = int(v.IntVal())
			case "rpc.grpc.status_code":
				// Ignored for spans.
			default:
				labels[k] = v.IntVal()
			}
		case pdata.AttributeValueTypeString:
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

			// messaging.*
			case "message_bus.destination", conventions.AttributeMessagingDestination:
				message.QueueName = stringval
				isMessagingSpan = true
			case conventions.AttributeMessagingOperation:
				messageOperation = stringval
				isMessagingSpan = true
			case conventions.AttributeMessagingSystem:
				messageSystem = stringval
				destinationService.Resource = stringval
				destinationService.Name = stringval
				isMessagingSpan = true

			// rpc.*
			//
			// TODO(axw) add RPC fieldset to ECS? Currently we drop these
			// attributes, and rely on the operation name and span type/subtype
			// like we do with Elastic APM agents.
			case conventions.AttributeRPCSystem:
				rpcSystem = stringval
				isRPCSpan = true
			case conventions.AttributeRPCService:
			case conventions.AttributeRPCMethod:

			// miscellaneous
			case "span.kind": // filter out
			case conventions.AttributePeerService:
				destinationService.Name = stringval
				if destinationService.Resource == "" {
					// Prefer using peer.address for resource.
					destinationService.Resource = stringval
				}
			case "component":
				component = stringval
				fallthrough
			default:
				labels[k] = stringval
			}
		}
		return true
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

	if isRPCSpan {
		// Set destination.service.* from the peer address, unless peer.service was specified.
		if destinationService.Name == "" {
			destHostPort := net.JoinHostPort(destAddr, strconv.Itoa(destPort))
			destinationService.Name = destHostPort
			destinationService.Resource = destHostPort
		}
	}

	switch {
	case isHTTPSpan:
		if http.StatusCode > 0 {
			if event.Outcome == outcomeUnknown {
				event.Outcome = clientHTTPStatusCodeOutcome(http.StatusCode)
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
		event.Subtype = messageSystem
		if messageOperation == "" && span.Kind() == pdata.SpanKindProducer {
			messageOperation = "send"
		}
		event.Action = messageOperation
		if destinationService.Resource != "" && message.QueueName != "" {
			destinationService.Resource += "/" + message.QueueName
		}
		event.Message = &message
	case isRPCSpan:
		event.Type = "external"
		event.Subtype = rpcSystem
	default:
		event.Type = "app"
		event.Subtype = component
	}

	if destAddr != "" {
		event.Destination = &model.Destination{Address: destAddr, Port: destPort}
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
	} else {
		event.RepresentativeCount = 1
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
		case pdata.AttributeValueTypeBool:
			labels["sampler_param"] = samplerParam.BoolVal()
		case pdata.AttributeValueTypeDouble:
			labels["sampler_param"] = samplerParam.DoubleVal()
		}
	}
}

func convertSpanEvent(
	logger *logp.Logger,
	event pdata.SpanEvent,
	metadata model.Metadata,
	transaction *model.Transaction, span *model.Span, // only one is non-nil
	timeDelta time.Duration,
	out *model.Batch,
) {
	var e *model.Error
	isJaeger := strings.HasPrefix(metadata.Service.Agent.Name, "Jaeger")
	if isJaeger {
		e = convertJaegerErrorSpanEvent(logger, event)
	} else {
		// Translate exception span events to errors.
		//
		// If it's not Jaeger, we assume OpenTelemetry semantic conventions.
		//
		// TODO(axw) we don't currently support arbitrary events, we only look
		// for exceptions and convert those to Elastic APM error events.
		if event.Name() != "exception" {
			// Per OpenTelemetry semantic conventions:
			//   `The name of the event MUST be "exception"`
			return
		}
		var exceptionEscaped bool
		var exceptionMessage, exceptionStacktrace, exceptionType string
		event.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			switch k {
			case conventions.AttributeExceptionMessage:
				exceptionMessage = v.StringVal()
			case conventions.AttributeExceptionStacktrace:
				exceptionStacktrace = v.StringVal()
			case conventions.AttributeExceptionType:
				exceptionType = v.StringVal()
			case "exception.escaped":
				exceptionEscaped = v.BoolVal()
			}
			return true
		})
		if exceptionMessage == "" && exceptionType == "" {
			// Per OpenTelemetry semantic conventions:
			//   `At least one of the following sets of attributes is required:
			//   - exception.type
			//   - exception.message`
			return
		}
		timestamp := event.Timestamp().AsTime()
		timestamp = timestamp.Add(timeDelta)
		e = convertOpenTelemetryExceptionSpanEvent(
			timestamp,
			exceptionType, exceptionMessage, exceptionStacktrace,
			exceptionEscaped, metadata.Service.Language.Name,
		)
	}
	if e != nil {
		if transaction != nil {
			addTransactionCtxToErr(transaction, e)
		}
		if span != nil {
			addSpanCtxToErr(span, e)
		}
		out.Errors = append(out.Errors, e)
	}
}

func convertJaegerErrorSpanEvent(logger *logp.Logger, event pdata.SpanEvent) *model.Error {
	var isError bool
	var exMessage, exType string
	logMessage := event.Name()
	hasMinimalInfo := logMessage != ""
	event.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		if v.Type() != pdata.AttributeValueTypeString {
			return true
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
		return true
	})
	if !isError {
		return nil
	}
	if !hasMinimalInfo {
		logger.Debugf("Cannot convert span event (name=%q) into elastic apm error: %v", event.Name())
		return nil
	}
	e := &model.Error{
		Timestamp: event.Timestamp().AsTime(),
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
	return e
}

func addTransactionCtxToErr(transaction *model.Transaction, err *model.Error) {
	err.Metadata = transaction.Metadata
	err.TransactionID = transaction.ID
	err.TraceID = transaction.TraceID
	err.ParentID = transaction.ID
	err.HTTP = transaction.HTTP
	err.URL = transaction.URL
	err.Page = transaction.Page
	err.Custom = transaction.Custom
	err.TransactionSampled = transaction.Sampled
	err.TransactionType = transaction.Type
}

func addSpanCtxToErr(span *model.Span, err *model.Error) {
	err.Metadata = span.Metadata
	err.TraceID = span.TraceID
	err.ParentID = span.ID
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
