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
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"google.golang.org/grpc/codes"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/datastreams"
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

	// TODO: handle net.host.connection.subtype, which will
	// require adding a new field to the model as well.

	attributeNetworkConnectionType    = "net.host.connection.type"
	attributeNetworkConnectionSubtype = "net.host.connection.subtype"
	attributeNetworkMCC               = "net.host.carrier.mcc"
	attributeNetworkMNC               = "net.host.carrier.mnc"
	attributeNetworkCarrierName       = "net.host.carrier.name"
	attributeNetworkICC               = "net.host.carrier.icc"
)

var (
	jsonTracesMarshaler  = otlp.NewJSONTracesMarshaler()
	jsonMetricsMarshaler = otlp.NewJSONMetricsMarshaler()
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
	logger := logp.NewLogger(logs.Otel)
	if logger.IsDebug() {
		data, err := jsonTracesMarshaler.MarshalTraces(traces)
		if err != nil {
			logger.Debug(err)
		} else {
			logger.Debug(string(data))
		}
	}
	batch := c.convert(traces, receiveTimestamp, logger)
	return c.Processor.ProcessBatch(ctx, batch)
}

func (c *Consumer) convert(td pdata.Traces, receiveTimestamp time.Time, logger *logp.Logger) *model.Batch {
	batch := model.Batch{}
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		c.convertResourceSpans(resourceSpans.At(i), receiveTimestamp, logger, &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceSpans(
	resourceSpans pdata.ResourceSpans,
	receiveTimestamp time.Time,
	logger *logp.Logger,
	out *model.Batch,
) {
	var baseEvent model.APMEvent
	var timeDelta time.Duration
	resource := resourceSpans.Resource()
	translateResourceMetadata(resource, &baseEvent)
	if exportTimestamp, ok := exportTimestamp(resource); ok {
		timeDelta = receiveTimestamp.Sub(exportTimestamp)
	}
	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans()
	for i := 0; i < instrumentationLibrarySpans.Len(); i++ {
		c.convertInstrumentationLibrarySpans(
			instrumentationLibrarySpans.At(i), baseEvent, timeDelta, logger, out,
		)
	}
}

func (c *Consumer) convertInstrumentationLibrarySpans(
	in pdata.InstrumentationLibrarySpans,
	baseEvent model.APMEvent,
	timeDelta time.Duration,
	logger *logp.Logger,
	out *model.Batch,
) {
	otelSpans := in.Spans()
	for i := 0; i < otelSpans.Len(); i++ {
		c.convertSpan(otelSpans.At(i), in.InstrumentationLibrary(), baseEvent, timeDelta, logger, out)
	}
}

func (c *Consumer) convertSpan(
	otelSpan pdata.Span,
	otelLibrary pdata.InstrumentationLibrary,
	baseEvent model.APMEvent,
	timeDelta time.Duration,
	logger *logp.Logger,
	out *model.Batch,
) {
	root := otelSpan.ParentSpanID().IsEmpty()
	var parentID string
	if !root {
		parentID = otelSpan.ParentSpanID().HexString()
	}

	startTime := otelSpan.StartTimestamp().AsTime()
	endTime := otelSpan.EndTimestamp().AsTime()
	duration := endTime.Sub(startTime)

	// Message consumption results in either a transaction or a span based
	// on whether the consumption is active or passive. Otel spans
	// currently do not have the metadata to make this distinction. For
	// now, we assume that the majority of consumption is passive, and
	// therefore start a transaction whenever span kind == consumer.
	name := otelSpan.Name()
	spanID := otelSpan.SpanID().HexString()
	event := baseEvent
	event.Labels = initEventLabels(event.Labels)
	event.Timestamp = startTime.Add(timeDelta)
	event.Trace.ID = otelSpan.TraceID().HexString()
	event.Event.Duration = duration
	event.Event.Outcome = spanStatusOutcome(otelSpan.Status())
	event.Parent.ID = parentID
	if root || otelSpan.Kind() == pdata.SpanKindServer || otelSpan.Kind() == pdata.SpanKindConsumer {
		event.Processor = model.TransactionProcessor
		event.Transaction = &model.Transaction{
			ID:      spanID,
			Name:    name,
			Sampled: true,
		}
		translateTransaction(otelSpan, otelLibrary, &event)
	} else {
		event.Processor = model.SpanProcessor
		event.Span = &model.Span{
			ID:   spanID,
			Name: name,
		}
		translateSpan(otelSpan, &event)
	}
	if len(event.Labels) == 0 {
		event.Labels = nil
	}
	*out = append(*out, event)

	events := otelSpan.Events()
	event.Labels = baseEvent.Labels         // only copy common labels to span events
	event.Event.Outcome = ""                // don't set event.outcome for span events
	event.Destination = model.Destination{} // don't set destination for span events
	for i := 0; i < events.Len(); i++ {
		*out = append(*out, convertSpanEvent(logger, events.At(i), event, timeDelta))
	}
}

func translateTransaction(
	span pdata.Span,
	library pdata.InstrumentationLibrary,
	event *model.APMEvent,
) {
	isJaeger := strings.HasPrefix(event.Agent.Name, "Jaeger")

	var (
		netHostName string
		netHostPort int
	)

	var (
		isHTTP         bool
		httpScheme     string
		httpURL        string
		httpServerName string
		httpHost       string
		http           model.HTTP
		httpRequest    model.HTTPRequest
		httpResponse   model.HTTPResponse
	)

	var (
		isMessaging bool
		message     model.Message
	)

	var component string
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
		case pdata.AttributeValueTypeArray:
			event.Labels[k] = ifaceAnyValueArray(v.ArrayVal())
		case pdata.AttributeValueTypeBool:
			event.Labels[k] = v.BoolVal()
		case pdata.AttributeValueTypeDouble:
			event.Labels[k] = v.DoubleVal()
		case pdata.AttributeValueTypeInt:
			switch kDots {
			case semconv.AttributeHTTPStatusCode:
				isHTTP = true
				httpResponse.StatusCode = int(v.IntVal())
				http.Response = &httpResponse
			case semconv.AttributeNetPeerPort:
				event.Source.Port = int(v.IntVal())
			case semconv.AttributeNetHostPort:
				netHostPort = int(v.IntVal())
			case "rpc.grpc.status_code":
				event.Transaction.Result = codes.Code(v.IntVal()).String()
			default:
				event.Labels[k] = v.IntVal()
			}
		case pdata.AttributeValueTypeString:
			stringval := truncate(v.StringVal())
			switch kDots {
			// http.*
			case semconv.AttributeHTTPMethod:
				isHTTP = true
				httpRequest.Method = stringval
				http.Request = &httpRequest
			case semconv.AttributeHTTPURL, semconv.AttributeHTTPTarget, "http.path":
				isHTTP = true
				httpURL = stringval
			case semconv.AttributeHTTPHost:
				isHTTP = true
				httpHost = stringval
			case semconv.AttributeHTTPScheme:
				isHTTP = true
				httpScheme = stringval
			case semconv.AttributeHTTPStatusCode:
				if intv, err := strconv.Atoi(stringval); err == nil {
					isHTTP = true
					httpResponse.StatusCode = intv
					http.Response = &httpResponse
				}
			case "http.protocol":
				if !strings.HasPrefix(stringval, "HTTP/") {
					// Unexpected, store in labels for debugging.
					event.Labels[k] = stringval
					break
				}
				stringval = strings.TrimPrefix(stringval, "HTTP/")
				fallthrough
			case semconv.AttributeHTTPFlavor:
				isHTTP = true
				http.Version = stringval
			case semconv.AttributeHTTPServerName:
				isHTTP = true
				httpServerName = stringval
			case semconv.AttributeHTTPClientIP:
				event.Client.IP = net.ParseIP(stringval)
			case semconv.AttributeHTTPUserAgent:
				event.UserAgent.Original = stringval

			// net.*
			case semconv.AttributeNetPeerIP:
				event.Source.IP = net.ParseIP(stringval)
			case semconv.AttributeNetPeerName:
				event.Source.Domain = stringval
			case semconv.AttributeNetHostName:
				netHostName = stringval
			case attributeNetworkConnectionType:
				event.Network.Connection.Type = stringval
			case attributeNetworkConnectionSubtype:
				event.Network.Connection.Subtype = stringval
			case attributeNetworkMCC:
				event.Network.Carrier.MCC = stringval
			case attributeNetworkMNC:
				event.Network.Carrier.MNC = stringval
			case attributeNetworkCarrierName:
				event.Network.Carrier.Name = stringval
			case attributeNetworkICC:
				event.Network.Carrier.ICC = stringval

			// messaging.*
			case "message_bus.destination", semconv.AttributeMessagingDestination:
				message.QueueName = stringval
				isMessaging = true

			// rpc.*
			//
			// TODO(axw) add RPC fieldset to ECS? Currently we drop these
			// attributes, and rely on the operation name like we do with
			// Elastic APM agents.
			case semconv.AttributeRPCSystem:
				event.Transaction.Type = "request"
			case semconv.AttributeRPCService:
			case semconv.AttributeRPCMethod:

			// miscellaneous
			case "span.kind": // filter out
			case "type":
				event.Transaction.Type = stringval
			case semconv.AttributeServiceVersion:
				// NOTE support for sending service.version as a span tag
				// is deprecated, and will be removed in 8.0. Instrumentation
				// should set this as a resource attribute (OTel) or tracer
				// tag (Jaeger).
				event.Service.Version = stringval
			case "component":
				component = stringval
				fallthrough
			default:
				event.Labels[k] = stringval
			}
		}
		return true
	})

	if event.Transaction.Type == "" {
		if isHTTP {
			event.Transaction.Type = "request"
		} else if isMessaging {
			event.Transaction.Type = "messaging"
		} else if component != "" {
			event.Transaction.Type = component
		} else {
			event.Transaction.Type = "custom"
		}
	}

	if isHTTP {
		event.HTTP = http

		// Set outcome nad result from status code.
		if statusCode := httpResponse.StatusCode; statusCode > 0 {
			if event.Event.Outcome == outcomeUnknown {
				event.Event.Outcome = serverHTTPStatusCodeOutcome(statusCode)
			}
			if event.Transaction.Result == "" {
				event.Transaction.Result = httpStatusCodeResult(statusCode)
			}
		}

		// Build the model.URL from http{URL,Host,Scheme}.
		httpHost := httpHost
		if httpHost == "" {
			httpHost = httpServerName
			if httpHost == "" {
				httpHost = netHostName
				if httpHost == "" {
					httpHost = event.Host.Hostname
				}
			}
			if httpHost != "" && netHostPort > 0 {
				httpHost = net.JoinHostPort(httpHost, strconv.Itoa(netHostPort))
			}
		}
		event.URL = model.ParseURL(httpURL, httpHost, httpScheme)
	}

	if isMessaging {
		event.Transaction.Message = &message
	}

	if event.Client.IP == nil {
		event.Client = model.Client(event.Source)
	}

	if samplerType != (pdata.AttributeValue{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate span metrics.
		parseSamplerAttributes(samplerType, samplerParam, &event.Transaction.RepresentativeCount, event.Labels)
	} else {
		event.Transaction.RepresentativeCount = 1
	}

	if event.Transaction.Result == "" {
		event.Transaction.Result = spanStatusResult(span.Status())
	}
	if name := library.Name(); name != "" {
		event.Service.Framework.Name = name
		event.Service.Framework.Version = library.Version()
	}
}

func translateSpan(span pdata.Span, event *model.APMEvent) {
	isJaeger := strings.HasPrefix(event.Agent.Name, "Jaeger")

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
	var httpRequest model.HTTPRequest
	var httpResponse model.HTTPResponse
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
		case pdata.AttributeValueTypeArray:
			event.Labels[k] = ifaceAnyValueArray(v.ArrayVal())
		case pdata.AttributeValueTypeBool:
			event.Labels[k] = v.BoolVal()
		case pdata.AttributeValueTypeDouble:
			event.Labels[k] = v.DoubleVal()
		case pdata.AttributeValueTypeInt:
			switch kDots {
			case "http.status_code":
				httpResponse.StatusCode = int(v.IntVal())
				http.Response = &httpResponse
				isHTTPSpan = true
			case semconv.AttributeNetPeerPort, "peer.port":
				netPeerPort = int(v.IntVal())
			case "rpc.grpc.status_code":
				// Ignored for spans.
			default:
				event.Labels[k] = v.IntVal()
			}
		case pdata.AttributeValueTypeString:
			stringval := truncate(v.StringVal())
			switch kDots {
			// http.*
			case semconv.AttributeHTTPHost:
				httpHost = stringval
				isHTTPSpan = true
			case semconv.AttributeHTTPScheme:
				httpScheme = stringval
				isHTTPSpan = true
			case semconv.AttributeHTTPTarget:
				httpTarget = stringval
				isHTTPSpan = true
			case semconv.AttributeHTTPURL:
				httpURL = stringval
				isHTTPSpan = true
			case semconv.AttributeHTTPMethod:
				httpRequest.Method = stringval
				http.Request = &httpRequest
				isHTTPSpan = true

			// db.*
			case "sql.query":
				if db.Type == "" {
					db.Type = "sql"
				}
				fallthrough
			case semconv.AttributeDBStatement:
				db.Statement = stringval
				isDBSpan = true
			case semconv.AttributeDBName, "db.instance":
				db.Instance = stringval
				isDBSpan = true
			case semconv.AttributeDBSystem, "db.type":
				db.Type = stringval
				isDBSpan = true
			case semconv.AttributeDBUser:
				db.UserName = stringval
				isDBSpan = true

			// net.*
			case semconv.AttributeNetPeerName, "peer.hostname":
				netPeerName = stringval
			case semconv.AttributeNetPeerIP, "peer.ipv4", "peer.ipv6":
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
			case attributeNetworkConnectionType:
				event.Network.Connection.Type = stringval
			case attributeNetworkConnectionSubtype:
				event.Network.Connection.Subtype = stringval
			case attributeNetworkMCC:
				event.Network.Carrier.MCC = stringval
			case attributeNetworkMNC:
				event.Network.Carrier.MNC = stringval
			case attributeNetworkCarrierName:
				event.Network.Carrier.Name = stringval
			case attributeNetworkICC:
				event.Network.Carrier.ICC = stringval

			// messaging.*
			case "message_bus.destination", semconv.AttributeMessagingDestination:
				message.QueueName = stringval
				isMessagingSpan = true
			case semconv.AttributeMessagingOperation:
				messageOperation = stringval
				isMessagingSpan = true
			case semconv.AttributeMessagingSystem:
				messageSystem = stringval
				destinationService.Resource = stringval
				destinationService.Name = stringval
				isMessagingSpan = true

			// rpc.*
			//
			// TODO(axw) add RPC fieldset to ECS? Currently we drop these
			// attributes, and rely on the operation name and span type/subtype
			// like we do with Elastic APM agents.
			case semconv.AttributeRPCSystem:
				rpcSystem = stringval
				isRPCSpan = true
			case semconv.AttributeRPCService:
			case semconv.AttributeRPCMethod:

			// miscellaneous
			case "span.kind": // filter out
			case semconv.AttributePeerService:
				destinationService.Name = stringval
				if destinationService.Resource == "" {
					// Prefer using peer.address for resource.
					destinationService.Resource = stringval
				}
			case "component":
				component = stringval
				fallthrough
			default:
				event.Labels[k] = stringval
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
					if destPort > 0 {
						httpHost = net.JoinHostPort(httpHost, strconv.Itoa(destPort))
					}
				}
				fullURL.Host = httpHost
				httpURL = fullURL.String()
			}
		}
		if fullURL != nil {
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
		if httpResponse.StatusCode > 0 {
			if event.Event.Outcome == outcomeUnknown {
				event.Event.Outcome = clientHTTPStatusCodeOutcome(httpResponse.StatusCode)
			}
		}
		event.Span.Type = "external"
		subtype := "http"
		event.Span.Subtype = subtype
		event.HTTP = http
		event.URL.Original = httpURL
	case isDBSpan:
		event.Span.Type = "db"
		if db.Type != "" {
			event.Span.Subtype = db.Type
			if destinationService.Name == "" {
				// For database requests, we currently just identify
				// the destination service by db.system.
				destinationService.Name = event.Span.Subtype
				destinationService.Resource = event.Span.Subtype
			}
		}
		event.Span.DB = &db
	case isMessagingSpan:
		event.Span.Type = "messaging"
		event.Span.Subtype = messageSystem
		if messageOperation == "" && span.Kind() == pdata.SpanKindProducer {
			messageOperation = "send"
		}
		event.Span.Action = messageOperation
		if destinationService.Resource != "" && message.QueueName != "" {
			destinationService.Resource += "/" + message.QueueName
		}
		event.Span.Message = &message
	case isRPCSpan:
		event.Span.Type = "external"
		event.Span.Subtype = rpcSystem
	default:
		event.Span.Type = "app"
		event.Span.Subtype = component
	}

	if destAddr != "" {
		event.Destination = model.Destination{Address: destAddr, Port: destPort}
	}
	if destinationService != (model.DestinationService{}) {
		if destinationService.Type == "" {
			// Copy span type to destination.service.type.
			destinationService.Type = event.Span.Type
		}
		event.Span.DestinationService = &destinationService
	}

	if samplerType != (pdata.AttributeValue{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate transaction metrics.
		parseSamplerAttributes(samplerType, samplerParam, &event.Span.RepresentativeCount, event.Labels)
	} else {
		event.Span.RepresentativeCount = 1
	}
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
	spanEvent pdata.SpanEvent,
	parent model.APMEvent, // either span or transaction
	timeDelta time.Duration,
) model.APMEvent {
	event := parent
	event.Labels = initEventLabels(event.Labels)
	event.Transaction = nil
	event.Span = nil
	event.Timestamp = spanEvent.Timestamp().AsTime().Add(timeDelta)

	isJaeger := strings.HasPrefix(parent.Agent.Name, "Jaeger")
	if isJaeger {
		event.Error = convertJaegerErrorSpanEvent(logger, spanEvent, event.Labels)
	} else if spanEvent.Name() == "exception" {
		// Translate exception span events to errors.
		//
		// If it's not Jaeger, we assume OpenTelemetry semantic semconv.
		// Per OpenTelemetry semantic conventions:
		//   `The name of the event MUST be "exception"`
		var exceptionEscaped bool
		var exceptionMessage, exceptionStacktrace, exceptionType string
		spanEvent.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			switch k {
			case semconv.AttributeExceptionMessage:
				exceptionMessage = v.StringVal()
			case semconv.AttributeExceptionStacktrace:
				exceptionStacktrace = v.StringVal()
			case semconv.AttributeExceptionType:
				exceptionType = v.StringVal()
			case "exception.escaped":
				exceptionEscaped = v.BoolVal()
			default:
				event.Labels[replaceDots(k)] = ifaceAttributeValue(v)
			}
			return true
		})
		if exceptionMessage != "" || exceptionType != "" {
			// Per OpenTelemetry semantic conventions:
			//   `At least one of the following sets of attributes is required:
			//   - exception.type
			//   - exception.message`
			event.Error = convertOpenTelemetryExceptionSpanEvent(
				exceptionType, exceptionMessage, exceptionStacktrace,
				exceptionEscaped, parent.Service.Language.Name,
			)
		}
	}

	if event.Error != nil {
		event.Processor = model.ErrorProcessor
		setErrorContext(&event, parent)
	} else {
		event.Processor = model.LogProcessor
		event.DataStream.Type = datastreams.LogsType
		event.Message = spanEvent.Name()
		spanEvent.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			event.Labels[replaceDots(k)] = ifaceAttributeValue(v)
			return true
		})
	}
	return event
}

func convertJaegerErrorSpanEvent(logger *logp.Logger, event pdata.SpanEvent, labels common.MapStr) *model.Error {
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
		default:
			labels[replaceDots(k)] = ifaceAttributeValue(v)
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
	e := &model.Error{}
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

func setErrorContext(out *model.APMEvent, parent model.APMEvent) {
	out.Trace.ID = parent.Trace.ID
	out.HTTP = parent.HTTP
	out.URL = parent.URL
	if parent.Transaction != nil {
		out.Transaction = &model.Transaction{
			ID:      parent.Transaction.ID,
			Sampled: parent.Transaction.Sampled,
			Type:    parent.Transaction.Type,
		}
		out.Error.Custom = parent.Transaction.Custom
		out.Parent.ID = parent.Transaction.ID
	}
	if parent.Span != nil {
		out.Parent.ID = parent.Span.ID
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

var standardStatusCodeResults = [...]string{
	"HTTP 1xx",
	"HTTP 2xx",
	"HTTP 3xx",
	"HTTP 4xx",
	"HTTP 5xx",
}

// httpStatusCodeResult returns the transaction result value to use for the
// given HTTP status code.
func httpStatusCodeResult(statusCode int) string {
	switch i := statusCode / 100; i {
	case 1, 2, 3, 4, 5:
		return standardStatusCodeResults[i-1]
	}
	return fmt.Sprintf("HTTP %d", statusCode)
}

// serverHTTPStatusCodeOutcome returns the transaction outcome value to use for
// the given HTTP status code.
func serverHTTPStatusCodeOutcome(statusCode int) string {
	if statusCode >= 500 {
		return outcomeFailure
	}
	return outcomeSuccess
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
