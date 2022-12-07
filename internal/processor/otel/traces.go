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
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"google.golang.org/grpc/codes"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/logs"
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
	jsonTracesMarshaler  = &ptrace.JSONMarshaler{}
	jsonMetricsMarshaler = &pmetric.JSONMarshaler{}
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
func (c *Consumer) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
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

func (c *Consumer) convert(td ptrace.Traces, receiveTimestamp time.Time, logger *logp.Logger) *model.Batch {
	batch := model.Batch{}
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		c.convertResourceSpans(resourceSpans.At(i), receiveTimestamp, logger, &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceSpans(
	resourceSpans ptrace.ResourceSpans,
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
	scopeSpans := resourceSpans.ScopeSpans()
	for i := 0; i < scopeSpans.Len(); i++ {
		c.convertScopeSpans(scopeSpans.At(i), baseEvent, timeDelta, logger, out)
	}
}

func (c *Consumer) convertScopeSpans(
	in ptrace.ScopeSpans,
	baseEvent model.APMEvent,
	timeDelta time.Duration,
	logger *logp.Logger,
	out *model.Batch,
) {
	otelSpans := in.Spans()
	for i := 0; i < otelSpans.Len(); i++ {
		c.convertSpan(otelSpans.At(i), in.Scope(), baseEvent, timeDelta, logger, out)
	}
}

func (c *Consumer) convertSpan(
	otelSpan ptrace.Span,
	otelLibrary pcommon.InstrumentationScope,
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
	initEventLabels(&event)
	event.Timestamp = startTime.Add(timeDelta)
	event.Trace.ID = otelSpan.TraceID().HexString()
	event.Event.Duration = duration
	event.Event.Outcome = spanStatusOutcome(otelSpan.Status())
	event.Parent.ID = parentID
	if root || otelSpan.Kind() == ptrace.SpanKindServer || otelSpan.Kind() == ptrace.SpanKindConsumer {
		event.Processor = model.TransactionProcessor
		event.Transaction = &model.Transaction{
			ID:      spanID,
			Name:    name,
			Sampled: true,
		}
		TranslateTransaction(otelSpan.Attributes(), otelSpan.Status(), otelLibrary, &event)
	} else {
		event.Processor = model.SpanProcessor
		event.Span = &model.Span{
			ID:   spanID,
			Name: name,
		}
		TranslateSpan(otelSpan.Kind(), otelSpan.Attributes(), &event)
	}
	translateSpanLinks(&event, otelSpan.Links())
	if len(event.Labels) == 0 {
		event.Labels = nil
	}
	if len(event.NumericLabels) == 0 {
		event.NumericLabels = nil
	}
	*out = append(*out, event)

	events := otelSpan.Events()
	event.Labels = baseEvent.Labels               // only copy common labels to span events
	event.NumericLabels = baseEvent.NumericLabels // only copy common labels to span events
	event.Event = model.Event{}                   // don't copy event.* to span events
	event.Destination = model.Destination{}       // don't set destination for span events
	for i := 0; i < events.Len(); i++ {
		*out = append(*out, convertSpanEvent(logger, events.At(i), event, timeDelta))
	}
}

// TranslateTransaction converts incoming otlp/otel trace data into the
// expected elasticsearch format.
func TranslateTransaction(
	attributes pcommon.Map,
	spanStatus ptrace.Status,
	library pcommon.InstrumentationScope,
	event *model.APMEvent,
) {
	isJaeger := strings.HasPrefix(event.Agent.Name, "Jaeger")

	var (
		netHostName string
		netHostPort int
	)

	var (
		httpScheme     string
		httpURL        string
		httpServerName string
		httpHost       string
		http           model.HTTP
		httpRequest    model.HTTPRequest
		httpResponse   model.HTTPResponse
	)

	var foundSpanType int
	var message model.Message

	var samplerType, samplerParam pcommon.Value
	attributes.Range(func(kDots string, v pcommon.Value) bool {
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
		case pcommon.ValueTypeSlice:
			setLabel(k, event, ifaceAttributeValue(v))
		case pcommon.ValueTypeBool:
			setLabel(k, event, ifaceAttributeValue(v))
		case pcommon.ValueTypeDouble:
			setLabel(k, event, ifaceAttributeValue(v))
		case pcommon.ValueTypeInt:
			switch kDots {
			case semconv.AttributeHTTPStatusCode:
				foundSpanType = httpSpan
				httpResponse.StatusCode = int(v.Int())
				http.Response = &httpResponse
			case semconv.AttributeNetPeerPort:
				event.Source.Port = int(v.Int())
			case semconv.AttributeNetHostPort:
				netHostPort = int(v.Int())
			case "rpc.grpc.status_code":
				event.Transaction.Result = codes.Code(v.Int()).String()
			default:
				setLabel(k, event, ifaceAttributeValue(v))
			}
		case pcommon.ValueTypeMap:
		case pcommon.ValueTypeStr:
			stringval := truncate(v.Str())
			switch kDots {
			// http.*
			case semconv.AttributeHTTPMethod:
				foundSpanType = httpSpan
				httpRequest.Method = stringval
				http.Request = &httpRequest
			case semconv.AttributeHTTPURL, semconv.AttributeHTTPTarget, "http.path":
				foundSpanType = httpSpan
				httpURL = stringval
			case semconv.AttributeHTTPHost:
				foundSpanType = httpSpan
				httpHost = stringval
			case semconv.AttributeHTTPScheme:
				foundSpanType = httpSpan
				httpScheme = stringval
			case semconv.AttributeHTTPStatusCode:
				if intv, err := strconv.Atoi(stringval); err == nil {
					foundSpanType = httpSpan
					httpResponse.StatusCode = intv
					http.Response = &httpResponse
				}
			case "http.protocol":
				if !strings.HasPrefix(stringval, "HTTP/") {
					// Unexpected, store in labels for debugging.
					event.Labels.Set(k, stringval)
					break
				}
				stringval = strings.TrimPrefix(stringval, "HTTP/")
				fallthrough
			case semconv.AttributeHTTPFlavor:
				foundSpanType = httpSpan
				http.Version = stringval
			case semconv.AttributeHTTPServerName:
				foundSpanType = httpSpan
				httpServerName = stringval
			case semconv.AttributeHTTPClientIP:
				if ip, err := netip.ParseAddr(stringval); err == nil {
					event.Client.IP = ip
				}
			case semconv.AttributeHTTPUserAgent:
				event.UserAgent.Original = stringval

			// net.*
			case semconv.AttributeNetPeerIP:
				if ip, err := netip.ParseAddr(stringval); err == nil {
					event.Source.IP = ip
				}
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
				foundSpanType = messagingSpan

			// rpc.*
			//
			// TODO(axw) add RPC fieldset to ECS? Currently we drop these
			// attributes, and rely on the operation name like we do with
			// Elastic APM agents.
			case semconv.AttributeRPCSystem:
				event.Transaction.Type = "request"
			case semconv.AttributeRPCGRPCStatusCode:
				event.Transaction.Type = "request"
			case semconv.AttributeRPCService:
			case semconv.AttributeRPCMethod:

			// miscellaneous
			case "type":
				event.Transaction.Type = stringval
			case "session.id":
				event.Session.ID = stringval
			case semconv.AttributeServiceVersion:
				// NOTE support for sending service.version as a span tag
				// is deprecated, and will be removed in 8.0. Instrumentation
				// should set this as a resource attribute (OTel) or tracer
				// tag (Jaeger).
				event.Service.Version = stringval
			default:
				event.Labels.Set(k, stringval)
			}
		}
		return true
	})

	if event.Transaction.Type == "" {
		switch foundSpanType {
		case httpSpan:
			event.Transaction.Type = "request"
		case messagingSpan:
			event.Transaction.Type = "messaging"
		default:
			event.Transaction.Type = "unknown"
		}
	}

	switch foundSpanType {
	case httpSpan:
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
	case messagingSpan:
		event.Transaction.Message = &message
	}

	if !event.Client.IP.IsValid() {
		event.Client = model.Client{IP: event.Source.IP, Port: event.Source.Port, Domain: event.Source.Domain}
	}

	if samplerType != (pcommon.Value{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate span metrics.
		parseSamplerAttributes(samplerType, samplerParam, event)
	} else {
		event.Transaction.RepresentativeCount = 1
	}

	if event.Transaction.Result == "" {
		event.Transaction.Result = spanStatusResult(spanStatus)
	}
	if name := library.Name(); name != "" {
		event.Service.Framework.Name = name
		event.Service.Framework.Version = library.Version()
	}
}

const (
	_ int = iota
	dbSpan
	httpSpan
	messagingSpan
	rpcSpan
)

// TranslateSpan converts incoming otlp/otel trace data into the
// expected elasticsearch format.
func TranslateSpan(spanKind ptrace.SpanKind, attributes pcommon.Map, event *model.APMEvent) {
	isJaeger := strings.HasPrefix(event.Agent.Name, "Jaeger")

	var (
		netPeerName string
		netPeerIP   string
		netPeerPort int
	)

	var (
		peerService string
		peerAddress string
	)

	var (
		httpURL    string
		httpHost   string
		httpTarget string
		httpScheme = "http"
	)

	var (
		messageSystem          string
		messageOperation       string
		messageTempDestination bool
	)

	var (
		rpcSystem  string
		rpcService string
	)

	var http model.HTTP
	var httpRequest model.HTTPRequest
	var httpResponse model.HTTPResponse
	var message model.Message
	var db model.DB
	var destinationService model.DestinationService
	var serviceTarget model.ServiceTarget
	var foundSpanType int
	var samplerType, samplerParam pcommon.Value
	attributes.Range(func(kDots string, v pcommon.Value) bool {
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
		case pcommon.ValueTypeSlice:
			setLabel(k, event, ifaceAttributeValueSlice(v.Slice()))
		case pcommon.ValueTypeBool:
			switch kDots {
			case semconv.AttributeMessagingTempDestination:
				messageTempDestination = v.Bool()
				fallthrough
			default:
				setLabel(k, event, strconv.FormatBool(v.Bool()))
			}
		case pcommon.ValueTypeDouble:
			setLabel(k, event, v.Double())
		case pcommon.ValueTypeInt:
			switch kDots {
			case "http.status_code":
				httpResponse.StatusCode = int(v.Int())
				http.Response = &httpResponse
				foundSpanType = httpSpan
			case semconv.AttributeNetPeerPort, "peer.port":
				netPeerPort = int(v.Int())
			case "rpc.grpc.status_code":
				// Ignored for spans.
			default:
				setLabel(k, event, v.Int())
			}
		case pcommon.ValueTypeStr:
			stringval := truncate(v.Str())

			switch kDots {
			// http.*
			case semconv.AttributeHTTPHost:
				httpHost = stringval
				foundSpanType = httpSpan
			case semconv.AttributeHTTPScheme:
				httpScheme = stringval
				foundSpanType = httpSpan
			case semconv.AttributeHTTPTarget:
				httpTarget = stringval
				foundSpanType = httpSpan
			case semconv.AttributeHTTPURL:
				httpURL = stringval
				foundSpanType = httpSpan
			case semconv.AttributeHTTPMethod:
				httpRequest.Method = stringval
				http.Request = &httpRequest
				foundSpanType = httpSpan

			// db.*
			case "sql.query":
				if db.Type == "" {
					db.Type = "sql"
				}
				fallthrough
			case semconv.AttributeDBStatement:
				// Statement should not be truncated, use original string value.
				db.Statement = v.Str()
				foundSpanType = dbSpan
			case semconv.AttributeDBName, "db.instance":
				db.Instance = stringval
				foundSpanType = dbSpan
			case semconv.AttributeDBSystem, "db.type":
				db.Type = stringval
				foundSpanType = dbSpan
			case semconv.AttributeDBUser:
				db.UserName = stringval
				foundSpanType = dbSpan

			// net.*
			case semconv.AttributeNetPeerName, "peer.hostname":
				netPeerName = stringval
			case semconv.AttributeNetPeerIP, "peer.ipv4", "peer.ipv6":
				netPeerIP = stringval
			case "peer.address":
				peerAddress = stringval
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

			// session.*
			case "session.id":
				event.Session.ID = stringval

			// messaging.*
			case "message_bus.destination", semconv.AttributeMessagingDestination:
				message.QueueName = stringval
				foundSpanType = messagingSpan
			case semconv.AttributeMessagingOperation:
				messageOperation = stringval
				foundSpanType = messagingSpan
			case semconv.AttributeMessagingSystem:
				messageSystem = stringval
				foundSpanType = messagingSpan

			// rpc.*
			//
			// TODO(axw) add RPC fieldset to ECS? Currently we drop these
			// attributes, and rely on the operation name and span type/subtype
			// like we do with Elastic APM agents.
			case semconv.AttributeRPCSystem:
				rpcSystem = stringval
				foundSpanType = rpcSpan
			case semconv.AttributeRPCService:
				rpcService = stringval
				foundSpanType = rpcSpan
			case semconv.AttributeRPCGRPCStatusCode:
				rpcSystem = "grpc"
				foundSpanType = rpcSpan
			case semconv.AttributeRPCMethod:

			// miscellaneous
			case "span.kind": // filter out
			case semconv.AttributePeerService:
				peerService = stringval
			default:
				event.Labels.Set(k, stringval)
			}
		}
		return true
	})

	if netPeerName == "" && (!strings.ContainsRune(peerAddress, ':') || net.ParseIP(peerAddress) != nil) {
		// peer.address is not necessarily a hostname
		// or IP address; it could be something like
		// a JDBC connection string or ip:port. Ignore
		// values containing colons, except for IPv6.
		netPeerName = peerAddress
	}

	destPort := netPeerPort
	destAddr := netPeerName
	if destAddr == "" {
		destAddr = netPeerIP
	}

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
		var port int
		portString := fullURL.Port()
		if portString != "" {
			port, _ = strconv.Atoi(portString)
		} else {
			port = schemeDefaultPort(fullURL.Scheme)
		}

		// Set destination.{address,port} from the HTTP URL,
		// replacing peer.* based values to ensure consistency.
		destAddr = truncate(fullURL.Hostname())
		if port > 0 {
			destPort = port
		}
	}

	serviceTarget.Name = peerService
	destinationService.Name = peerService
	destinationService.Resource = peerService
	if peerAddress != "" {
		destinationService.Resource = peerAddress
	}

	switch foundSpanType {
	case httpSpan:
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
		serviceTarget.Type = event.Span.Subtype
		if fullURL != nil {
			url := url.URL{Scheme: fullURL.Scheme, Host: fullURL.Host}
			resource := url.Host
			if destPort == schemeDefaultPort(url.Scheme) {
				if fullURL.Port() != "" {
					// Remove the default port from destination.service.name
					url.Host = destAddr
				} else {
					// Add the default port to destination.service.resource
					resource = fmt.Sprintf("%s:%d", resource, destPort)
				}
			}

			serviceTarget.Name = resource
			if destinationService.Name == "" {
				destinationService.Name = url.String()
				destinationService.Resource = resource
			}
		}
	case dbSpan:
		event.Span.Type = "db"
		event.Span.Subtype = db.Type
		serviceTarget.Type = event.Span.Type
		if event.Span.Subtype != "" {
			serviceTarget.Type = event.Span.Subtype
			if destinationService.Name == "" {
				// For database requests, we currently just identify
				// the destination service by db.system.
				destinationService.Name = event.Span.Subtype
				destinationService.Resource = event.Span.Subtype
			}
		}
		if db.Instance != "" {
			serviceTarget.Name = db.Instance
		}
		event.Span.DB = &db
	case messagingSpan:
		event.Span.Type = "messaging"
		event.Span.Subtype = messageSystem
		if messageOperation == "" && spanKind == ptrace.SpanKindProducer {
			messageOperation = "send"
		}
		event.Span.Action = messageOperation
		serviceTarget.Type = event.Span.Type
		if event.Span.Subtype != "" {
			serviceTarget.Type = event.Span.Subtype
			if destinationService.Name == "" {
				destinationService.Name = event.Span.Subtype
				destinationService.Resource = event.Span.Subtype
			}
		}
		if destinationService.Resource != "" && message.QueueName != "" {
			destinationService.Resource += "/" + message.QueueName
		}
		if message.QueueName != "" && !messageTempDestination {
			serviceTarget.Name = message.QueueName
		}
		event.Span.Message = &message
	case rpcSpan:
		event.Span.Type = "external"
		event.Span.Subtype = rpcSystem
		serviceTarget.Type = event.Span.Type
		if event.Span.Subtype != "" {
			serviceTarget.Type = event.Span.Subtype
		}
		// Set destination.service.* from the peer address, unless peer.service was specified.
		if destinationService.Name == "" {
			destHostPort := net.JoinHostPort(destAddr, strconv.Itoa(destPort))
			destinationService.Name = destHostPort
			destinationService.Resource = destHostPort
		}
		if rpcService != "" {
			serviceTarget.Name = rpcService
		}
	default:
		// Only set event.Span.Type if not already set
		if event.Span.Type == "" {
			switch spanKind {
			case ptrace.SpanKindInternal:
				event.Span.Type = "app"
				event.Span.Subtype = "internal"
			default:
				event.Span.Type = "unknown"
			}
		}
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

	if serviceTarget != (model.ServiceTarget{}) {
		event.Service.Target = &serviceTarget
	}

	if samplerType != (pcommon.Value{}) {
		// The client has reported its sampling rate, so we can use it to extrapolate transaction metrics.
		parseSamplerAttributes(samplerType, samplerParam, event)
	} else {
		event.Span.RepresentativeCount = 1
	}
}

func parseSamplerAttributes(samplerType, samplerParam pcommon.Value, event *model.APMEvent) {
	switch samplerType := samplerType.Str(); samplerType {
	case "probabilistic":
		probability := samplerParam.Double()
		if probability > 0 && probability <= 1 {
			if event.Span != nil {
				event.Span.RepresentativeCount = 1 / probability
			}
			if event.Transaction != nil {
				event.Transaction.RepresentativeCount = 1 / probability
			}
		}
	default:
		event.Labels.Set("sampler_type", samplerType)
		switch samplerParam.Type() {
		case pcommon.ValueTypeBool:
			event.Labels.Set("sampler_param", strconv.FormatBool(samplerParam.Bool()))
		case pcommon.ValueTypeDouble:
			event.NumericLabels.Set("sampler_param", samplerParam.Double())
		}
	}
}

func convertSpanEvent(
	logger *logp.Logger,
	spanEvent ptrace.SpanEvent,
	parent model.APMEvent, // either span or transaction
	timeDelta time.Duration,
) model.APMEvent {
	event := parent
	initEventLabels(&event)
	event.Transaction = nil
	event.Span = nil
	event.Timestamp = spanEvent.Timestamp().AsTime().Add(timeDelta)

	isJaeger := strings.HasPrefix(parent.Agent.Name, "Jaeger")
	if isJaeger {
		event.Error = convertJaegerErrorSpanEvent(logger, spanEvent, &event)
	} else if spanEvent.Name() == "exception" {
		// Translate exception span events to errors.
		//
		// If it's not Jaeger, we assume OpenTelemetry semantic semconv.
		// Per OpenTelemetry semantic conventions:
		//   `The name of the event MUST be "exception"`
		var exceptionEscaped bool
		var exceptionMessage, exceptionStacktrace, exceptionType string
		spanEvent.Attributes().Range(func(k string, v pcommon.Value) bool {
			switch k {
			case semconv.AttributeExceptionMessage:
				exceptionMessage = v.Str()
			case semconv.AttributeExceptionStacktrace:
				exceptionStacktrace = v.Str()
			case semconv.AttributeExceptionType:
				exceptionType = v.Str()
			case "exception.escaped":
				exceptionEscaped = v.Bool()
			default:
				setLabel(replaceDots(k), &event, ifaceAttributeValue(v))
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
		event.Message = spanEvent.Name()
		spanEvent.Attributes().Range(func(k string, v pcommon.Value) bool {
			k = replaceDots(k)
			if isJaeger && k == "message" {
				event.Message = truncate(v.Str())
				return true
			}
			setLabel(k, &event, ifaceAttributeValue(v))
			return true
		})
	}
	return event
}

func convertJaegerErrorSpanEvent(logger *logp.Logger, event ptrace.SpanEvent, apmEvent *model.APMEvent) *model.Error {
	var isError bool
	var exMessage, exType string
	var logMessage string

	if name := truncate(event.Name()); name == "error" {
		isError = true // according to opentracing spec
	} else {
		// Jaeger seems to send the message in the 'event' field.
		//
		// In case 'message' is sent we will use that, otherwise
		// we will use 'event'.
		logMessage = name
	}

	event.Attributes().Range(func(k string, v pcommon.Value) bool {
		if v.Type() != pcommon.ValueTypeStr {
			return true
		}
		stringval := truncate(v.Str())
		switch k {
		case "error", "error.object":
			exMessage = stringval
			isError = true
		case "error.kind":
			exType = stringval
			isError = true
		case "level":
			isError = stringval == "error"
		case "message":
			logMessage = stringval
		default:
			setLabel(replaceDots(k), apmEvent, ifaceAttributeValue(v))
		}
		return true
	})
	if !isError {
		return nil
	}
	if logMessage == "" && exMessage == "" && exType == "" {
		logger.Debugf("Cannot convert span event (name=%q) into elastic apm error: %v", event.Name())
		return nil
	}
	e := &model.Error{}
	if logMessage != "" {
		e.Log = &model.ErrorLog{Message: logMessage}
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

func translateSpanLinks(out *model.APMEvent, in ptrace.SpanLinkSlice) {
	n := in.Len()
	if n == 0 {
		return
	}
	if out.Span == nil {
		out.Span = &model.Span{}
	}
	out.Span.Links = make([]model.SpanLink, n)
	for i := 0; i < n; i++ {
		link := in.At(i)
		out.Span.Links[i] = model.SpanLink{
			Span:  model.Span{ID: link.SpanID().HexString()},
			Trace: model.Trace{ID: link.TraceID().HexString()},
		}
	}
}

func replaceDots(s string) string {
	return strings.ReplaceAll(s, dot, underscore)
}

// spanStatusOutcome returns the outcome for transactions and spans based on
// the given OTLP span status.
func spanStatusOutcome(status ptrace.Status) string {
	switch status.Code() {
	case ptrace.StatusCodeOk:
		return outcomeSuccess
	case ptrace.StatusCodeError:
		return outcomeFailure
	}
	return outcomeUnknown
}

// spanStatusResult returns the result for transactions based on the given
// OTLP span status. If the span status is unknown, an empty result string
// is returned.
func spanStatusResult(status ptrace.Status) string {
	switch status.Code() {
	case ptrace.StatusCodeOk:
		return "Success"
	case ptrace.StatusCodeError:
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
