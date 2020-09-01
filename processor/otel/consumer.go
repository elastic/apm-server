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
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
)

const (
	AgentNameJaeger = "Jaeger"

	sourceFormatJaeger = "jaeger"
	keywordLength      = 1024
	dot                = "."
	underscore         = "_"
)

// Consumer transforms open-telemetry data to be compatible with elastic APM data
type Consumer struct {
	Reporter publish.Reporter
}

// ConsumeTraceData consumes OpenTelemetry trace data,
// converting into Elastic APM events and reporting to the Elastic APM schema.
func (c *Consumer) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	batch := c.convert(td)
	return c.Reporter(ctx, publish.PendingReq{
		Transformables: batch.Transformables(),
		Trace:          true,
	})
}

func (c *Consumer) convert(td consumerdata.TraceData) *model.Batch {
	md := model.Metadata{}
	parseMetadata(td, &md)
	hostname := md.System.DetectedHostname

	logger := logp.NewLogger(logs.Otel)
	batch := model.Batch{}
	for _, otelSpan := range td.Spans {
		if otelSpan == nil {
			continue
		}

		root := len(otelSpan.ParentSpanId) == 0

		var parentID, spanID, traceID string
		if td.SourceFormat == sourceFormatJaeger {
			if !root {
				parentID = formatJaegerSpanID(otelSpan.ParentSpanId)
			}

			traceID = formatJaegerTraceID(otelSpan.TraceId)
			spanID = formatJaegerSpanID(otelSpan.SpanId)
		} else {
			if !root {
				parentID = fmt.Sprintf("%x", otelSpan.ParentSpanId)
			}

			traceID = fmt.Sprintf("%x", otelSpan.TraceId)
			spanID = fmt.Sprintf("%x", otelSpan.SpanId)
		}

		startTime := parseTimestamp(otelSpan.StartTime)
		var duration float64
		if otelSpan.EndTime != nil && !startTime.IsZero() {
			duration = parseTimestamp(otelSpan.EndTime).Sub(startTime).Seconds() * 1000
		}
		name := otelSpan.GetName().GetValue()
		if root || otelSpan.Kind == tracepb.Span_SERVER {
			transaction := model.Transaction{
				Metadata:  md,
				ID:        spanID,
				ParentID:  parentID,
				TraceID:   traceID,
				Timestamp: startTime,
				Duration:  duration,
				Name:      name,
			}
			parseTransaction(otelSpan, td.SourceFormat, hostname, &transaction)
			batch.Transactions = append(batch.Transactions, &transaction)
			for _, err := range parseErrors(logger, td.SourceFormat, otelSpan) {
				addTransactionCtxToErr(transaction, err)
				batch.Errors = append(batch.Errors, err)
			}

		} else {
			span := model.Span{
				Metadata:  md,
				ID:        spanID,
				ParentID:  parentID,
				TraceID:   traceID,
				Timestamp: startTime,
				Duration:  duration,
				Name:      name,
				Outcome:   "unknown",
			}
			parseSpan(otelSpan, td.SourceFormat, &span)
			batch.Spans = append(batch.Spans, &span)
			for _, err := range parseErrors(logger, td.SourceFormat, otelSpan) {
				addSpanCtxToErr(span, hostname, err)
				batch.Errors = append(batch.Errors, err)
			}
		}
	}
	return &batch
}

func parseMetadata(td consumerdata.TraceData, md *model.Metadata) {
	md.Service.Name = truncate(td.Node.GetServiceInfo().GetName())
	if md.Service.Name == "" {
		md.Service.Name = "unknown"
	}

	if ident := td.Node.GetIdentifier(); ident != nil {
		md.Process.Pid = int(ident.Pid)
		if hostname := truncate(ident.HostName); hostname != "" {
			md.System.DetectedHostname = hostname
		}
	}
	if languageName, ok := languageName[td.Node.GetLibraryInfo().GetLanguage()]; ok {
		md.Service.Language.Name = languageName
	}

	switch td.SourceFormat {
	case sourceFormatJaeger:
		// version is of format `Jaeger-<agentlanguage>-<version>`, e.g. `Jaeger-Go-2.20.0`
		nVersionParts := 3
		versionParts := strings.SplitN(td.Node.GetLibraryInfo().GetExporterVersion(), "-", nVersionParts)
		if md.Service.Language.Name == "" && len(versionParts) == nVersionParts {
			md.Service.Language.Name = versionParts[1]
		}
		if v := versionParts[len(versionParts)-1]; v != "" {
			md.Service.Agent.Version = v
		} else {
			md.Service.Agent.Version = "unknown"
		}
		agentName := AgentNameJaeger
		if md.Service.Language.Name != "" {
			agentName = truncate(agentName + "/" + md.Service.Language.Name)
		}
		md.Service.Agent.Name = agentName

		if attributes := td.Node.GetAttributes(); attributes != nil {
			if clientUUID, ok := attributes["client-uuid"]; ok {
				md.Service.Agent.EphemeralID = truncate(clientUUID)
				delete(td.Node.Attributes, "client-uuid")
			}
			if ip, ok := attributes["ip"]; ok {
				md.System.IP = utility.ParseIP(ip)
				delete(td.Node.Attributes, "ip")
			}
		}
	default:
		md.Service.Agent.Name = strings.Title(td.SourceFormat)
		md.Service.Agent.Version = "unknown"
	}

	if md.Service.Language.Name == "" {
		md.Service.Language.Name = "unknown"
	}

	md.Labels = make(common.MapStr)
	for key, val := range td.Node.GetAttributes() {
		md.Labels[key] = truncate(val)
	}
	if t := td.Resource.GetType(); t != "" {
		md.Labels["resource"] = truncate(t)
	}
	for key, val := range td.Resource.GetLabels() {
		md.Labels[key] = truncate(val)
	}
}

func parseTransaction(span *tracepb.Span, sourceFormat string, hostname string, event *model.Transaction) {
	labels := make(common.MapStr)
	var http model.Http
	var httpStatusCode int
	var message model.Message
	var component string
	var outcome, result string
	var hasFailed bool
	var isHTTP, isMessaging bool
	var samplerType, samplerParam *tracepb.AttributeValue
	for kDots, v := range span.Attributes.GetAttributeMap() {
		if sourceFormat == sourceFormatJaeger {
			switch kDots {
			case "sampler.type":
				samplerType = v
				continue
			case "sampler.param":
				samplerParam = v
				continue
			}
		}

		k := replaceDots(kDots)
		switch v := v.Value.(type) {
		case *tracepb.AttributeValue_BoolValue:
			utility.DeepUpdate(labels, k, v.BoolValue)
			if k == "error" {
				hasFailed = v.BoolValue
			}
		case *tracepb.AttributeValue_DoubleValue:
			utility.DeepUpdate(labels, k, v.DoubleValue)
		case *tracepb.AttributeValue_IntValue:
			switch kDots {
			case "http.status_code":
				httpStatusCode = int(v.IntValue)
				isHTTP = true
			default:
				utility.DeepUpdate(labels, k, v.IntValue)
			}
		case *tracepb.AttributeValue_StringValue:
			switch kDots {
			case "span.kind": // filter out
			case "http.method":
				http.Request = &model.Req{Method: truncate(v.StringValue.Value)}
				isHTTP = true
			case "http.url", "http.path":
				event.URL = model.ParseURL(v.StringValue.Value, hostname)
				isHTTP = true
			case "http.status_code":
				if intv, err := strconv.Atoi(v.StringValue.Value); err == nil {
					httpStatusCode = intv
				}
				isHTTP = true
			case "http.protocol":
				if strings.HasPrefix(v.StringValue.Value, "HTTP/") {
					version := truncate(strings.TrimPrefix(v.StringValue.Value, "HTTP/"))
					http.Version = &version
				} else {
					utility.DeepUpdate(labels, k, v.StringValue.Value)
				}
				isHTTP = true
			case "message_bus.destination":
				message.QueueName = &v.StringValue.Value
				isMessaging = true
			case "type":
				event.Type = truncate(v.StringValue.Value)
			case "service.version":
				event.Metadata.Service.Version = truncate(v.StringValue.Value)
			case "component":
				component = truncate(v.StringValue.Value)
				fallthrough
			default:
				utility.DeepUpdate(labels, k, truncate(v.StringValue.Value))
			}
		}
	}

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
		if httpStatusCode == 0 {
			httpStatusCode = int(span.GetStatus().GetCode())
		}
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
		if hasFailed {
			result = "Error"
			outcome = "failure"
		} else {
			result = "Success"
			outcome = "success"
		}
	}
	event.Result = result
	event.Outcome = outcome

	if samplerType != nil && samplerParam != nil {
		// The client has reported its sampling rate, so
		// we can use it to extrapolate span metrics.
		parseSamplerAttributes(samplerType, samplerParam, &event.RepresentativeCount, labels)
	}

	if len(labels) == 0 {
		return
	}
	l := model.Labels(labels)
	event.Labels = &l
}

func parseSpan(span *tracepb.Span, sourceFormat string, event *model.Span) {
	labels := make(common.MapStr)

	var http model.HTTP
	var message model.Message
	var db model.DB
	var destination model.Destination
	var destinationService model.DestinationService
	var isDBSpan, isHTTPSpan, isMessagingSpan bool
	var component string
	var samplerType, samplerParam *tracepb.AttributeValue
	for kDots, v := range span.Attributes.GetAttributeMap() {
		if sourceFormat == sourceFormatJaeger {
			switch kDots {
			case "sampler.type":
				samplerType = v
				continue
			case "sampler.param":
				samplerParam = v
				continue
			}
		}

		k := replaceDots(kDots)
		switch v := v.Value.(type) {
		case *tracepb.AttributeValue_BoolValue:
			utility.DeepUpdate(labels, k, v.BoolValue)
		case *tracepb.AttributeValue_DoubleValue:
			utility.DeepUpdate(labels, k, v.DoubleValue)
		case *tracepb.AttributeValue_IntValue:
			switch kDots {
			case "http.status_code":
				code := int(v.IntValue)
				http.StatusCode = &code
				isHTTPSpan = true
			case "peer.port":
				port := int(v.IntValue)
				destination.Port = &port
			default:
				utility.DeepUpdate(labels, k, v.IntValue)
			}
		case *tracepb.AttributeValue_StringValue:
			switch kDots {
			case "span.kind": // filter out
			case "http.url":
				url := truncate(v.StringValue.Value)
				http.URL = &url
				isHTTPSpan = true
			case "http.method":
				method := truncate(v.StringValue.Value)
				http.Method = &method
				isHTTPSpan = true
			case "sql.query":
				db.Statement = &v.StringValue.Value
				if db.Type == nil {
					dbType := "sql"
					db.Type = &dbType
				}
				isDBSpan = true
			case "db.statement":
				db.Statement = &v.StringValue.Value
				isDBSpan = true
			case "db.instance":
				val := truncate(v.StringValue.Value)
				db.Instance = &val
				isDBSpan = true
			case "db.type":
				val := truncate(v.StringValue.Value)
				db.Type = &val
				isDBSpan = true
			case "db.user":
				val := truncate(v.StringValue.Value)
				db.UserName = &val
				isDBSpan = true
			case "peer.address":
				val := truncate(v.StringValue.Value)
				destinationService.Resource = &val
				if !strings.ContainsRune(val, ':') || net.ParseIP(val) != nil {
					// peer.address is not necessarily a hostname
					// or IP address; it could be something like
					// a JDBC connection string or ip:port. Ignore
					// values containing colons, except for IPv6.
					destination.Address = &val
				}
			case "peer.hostname", "peer.ipv4", "peer.ipv6":
				val := truncate(v.StringValue.Value)
				destination.Address = &val
			case "peer.service":
				val := truncate(v.StringValue.Value)
				destinationService.Name = &val
				if destinationService.Resource == nil {
					// Prefer using peer.address for resource.
					destinationService.Resource = &val
				}
			case "message_bus.destination":
				val := truncate(v.StringValue.Value)
				message.QueueName = &val
				isMessagingSpan = true
			case "component":
				component = truncate(v.StringValue.Value)
				fallthrough
			default:
				utility.DeepUpdate(labels, k, truncate(v.StringValue.Value))
			}
		}
	}

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
		if http.StatusCode == nil {
			if code := int(span.GetStatus().GetCode()); code != 0 {
				http.StatusCode = &code
			}
		}
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

	if samplerType != nil && samplerParam != nil {
		// The client has reported its sampling rate, so
		// we can use it to extrapolate transaction metrics.
		parseSamplerAttributes(samplerType, samplerParam, &event.RepresentativeCount, labels)
	}

	if len(labels) == 0 {
		return
	}
	event.Labels = labels
}

func parseSamplerAttributes(samplerType, samplerParam *tracepb.AttributeValue, representativeCount *float64, labels common.MapStr) {
	switch samplerType.GetStringValue().GetValue() {
	case "probabilistic":
		probability := samplerParam.GetDoubleValue()
		if probability > 0 && probability <= 1 {
			*representativeCount = 1 / probability
		}
	default:
		utility.DeepUpdate(labels, "sampler_type", samplerType.GetStringValue().GetValue())
		switch v := samplerParam.Value.(type) {
		case *tracepb.AttributeValue_BoolValue:
			utility.DeepUpdate(labels, "sampler_param", v.BoolValue)
		case *tracepb.AttributeValue_DoubleValue:
			utility.DeepUpdate(labels, "sampler_param", v.DoubleValue)
		}
	}
}

func parseErrors(logger *logp.Logger, source string, otelSpan *tracepb.Span) []*model.Error {
	var errors []*model.Error
	for _, log := range otelSpan.GetTimeEvents().GetTimeEvent() {
		var isError, hasMinimalInfo bool
		var err model.Error
		var logMessage, exMessage, exType string
		for k, v := range log.GetAnnotation().GetAttributes().GetAttributeMap() {
			if source == sourceFormatJaeger {
				switch v := v.Value.(type) {
				case *tracepb.AttributeValue_StringValue:
					vStr := v.StringValue.Value
					switch k {
					case "error", "error.object":
						exMessage = vStr
						hasMinimalInfo = true
						isError = true
					case "event":
						if vStr == "error" { // according to opentracing spec
							isError = true
						} else if logMessage == "" {
							// jaeger seems to send the message in the 'event' field
							// in case 'event' and 'message' are sent, 'message' is used
							logMessage = vStr
							hasMinimalInfo = true
						}
					case "message":
						logMessage = vStr
						hasMinimalInfo = true
					case "error.kind":
						exType = vStr
						hasMinimalInfo = true
						isError = true
					case "level":
						isError = vStr == "error"
					}
				}
			}
		}
		if !isError {
			continue
		}
		if !hasMinimalInfo {
			if logger.IsDebug() {
				logger.Debugf("Cannot convert %s event into elastic apm error: %v", source, log)
			}
			continue
		}

		if logMessage != "" {
			err.Log = &model.Log{Message: logMessage}
		}
		if exMessage != "" || exType != "" {
			err.Exception = &model.Exception{}
			if exMessage != "" {
				err.Exception.Message = &exMessage
			}
			if exType != "" {
				err.Exception.Type = &exType
			}
		}
		err.Timestamp = parseTimestamp(log.GetTime())
		errors = append(errors, &err)
	}
	return errors
}

func addTransactionCtxToErr(transaction model.Transaction, err *model.Error) {
	err.Metadata = transaction.Metadata
	err.TransactionID = transaction.ID
	err.TraceID = transaction.TraceID
	err.ParentID = transaction.ID
	err.HTTP = transaction.HTTP
	err.URL = transaction.URL
	err.TransactionType = &transaction.Type
}

func addSpanCtxToErr(span model.Span, hostname string, err *model.Error) {
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

func parseTimestamp(timestampT *timestamp.Timestamp) time.Time {
	if timestampT == nil {
		return time.Time{}
	}
	return time.Unix(timestampT.Seconds, int64(timestampT.Nanos)).UTC()
}

var languageName = map[commonpb.LibraryInfo_Language]string{
	1:  "C++",
	2:  "CSharp",
	3:  "Erlang",
	4:  "Go",
	5:  "Java",
	6:  "Node",
	7:  "PHP",
	8:  "Python",
	9:  "Ruby",
	10: "JavaScript",
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
		return "failure"
	}
	return "success"
}

// clientStatusCodeOutcome returns the span outcome value to use for the given status code.
func clientStatusCodeOutcome(statusCode int) string {
	if statusCode >= 400 {
		return "failure"
	}
	return "success"
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
func formatJaegerTraceID(traceID []byte) string {
	jaegerTraceIDHigh, jaegerTraceIDLow, err := tracetranslator.BytesToUInt64TraceID(traceID)
	if err != nil {
		return fmt.Sprintf("%x", traceID)
	}

	if jaegerTraceIDHigh == 0 {
		return fmt.Sprintf("%x", jaegerTraceIDLow)
	}

	return fmt.Sprintf("%x%016x", jaegerTraceIDHigh, jaegerTraceIDLow)
}

// formatJaegerSpanID returns the spanID as string in Jaeger format (hexadecimal without leading zeros)
func formatJaegerSpanID(spanID []byte) string {
	jaegerSpanID, err := tracetranslator.BytesToUInt64SpanID(spanID)
	if err != nil {
		return fmt.Sprintf("%x", spanID)
	}

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
