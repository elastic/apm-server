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

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	model_error "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metadata"
	model_span "github.com/elastic/apm-server/model/span"
	model_transaction "github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
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
	TransformConfig transform.Config
	ModelConfig     model.Config
	Reporter        publish.Reporter
}

// ConsumeTraceData consumes OpenTelemetry trace data,
// converting into Elastic APM events and reporting to the Elastic APM schema.
func (c *Consumer) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	metadata, transformables := c.convert(td)
	transformContext := &transform.Context{
		Config:   c.TransformConfig,
		Metadata: metadata,
	}

	return c.Reporter(ctx, publish.PendingReq{
		Transformables: transformables,
		Tcontext:       transformContext,
		Trace:          true,
	})
}

func (c *Consumer) convert(td consumerdata.TraceData) (metadata.Metadata, []transform.Transformable) {
	md := metadata.Metadata{}
	parseMetadata(td, &md)
	var hostname string
	if md.System != nil && md.System.DetectedHostname != nil {
		hostname = *md.System.DetectedHostname
	}

	logger := logp.NewLogger(logs.Otel)
	transformables := make([]transform.Transformable, 0, len(td.Spans))
	for _, otelSpan := range td.Spans {
		if otelSpan == nil {
			continue
		}

		var parentID *string
		root := len(otelSpan.ParentSpanId) == 0
		if !root {
			str := fmt.Sprintf("%x", otelSpan.ParentSpanId)
			parentID = &str
		}
		traceID := fmt.Sprintf("%x", otelSpan.TraceId)
		spanID := fmt.Sprintf("%x", otelSpan.SpanId)
		startTime := parseTimestamp(otelSpan.StartTime)
		var duration float64
		if otelSpan.EndTime != nil && !startTime.IsZero() {
			duration = parseTimestamp(otelSpan.EndTime).Sub(startTime).Seconds() * 1000
		}
		name := otelSpan.GetName().GetValue()
		if root || otelSpan.Kind == tracepb.Span_SERVER {
			transaction := model_transaction.Event{
				Id:        spanID,
				ParentId:  parentID,
				TraceId:   traceID,
				Timestamp: startTime,
				Duration:  duration,
				Name:      &name,
			}
			parseTransaction(otelSpan, hostname, &transaction)
			transformables = append(transformables, &transaction)
			for _, err := range parseErrors(logger, td.SourceFormat, otelSpan) {
				addTransactionCtxToErr(transaction, err)
				transformables = append(transformables, err)
			}

		} else {
			span := model_span.Event{
				Id:        spanID,
				ParentId:  *parentID,
				TraceId:   traceID,
				Timestamp: startTime,
				Duration:  duration,
				Name:      name,
			}
			parseSpan(otelSpan, &span)
			transformables = append(transformables, &span)
			for _, err := range parseErrors(logger, td.SourceFormat, otelSpan) {
				addSpanCtxToErr(span, hostname, err)
				transformables = append(transformables, err)
			}
		}
	}
	return md, transformables
}

func parseMetadata(td consumerdata.TraceData, md *metadata.Metadata) {
	serviceName := td.Node.GetServiceInfo().GetName()
	if serviceName == "" {
		serviceName = "unknown"
	}
	serviceName = truncate(serviceName)
	md.Service = &metadata.Service{Name: &serviceName}

	if ident := td.Node.GetIdentifier(); ident != nil {
		if pid := ident.Pid; pid != 0 {
			md.Process = &metadata.Process{Pid: int(pid)}
		}
		if hostname := truncate(ident.HostName); hostname != "" {
			md.System = &metadata.System{DetectedHostname: &hostname}
		}
	}
	if languageName, ok := languageName[td.Node.GetLibraryInfo().GetLanguage()]; ok {
		md.Service.Language.Name = &languageName
	}

	switch td.SourceFormat {
	case sourceFormatJaeger:
		// version is of format `Jaeger-<agentlanguage>-<version>`, e.g. `Jaeger-Go-2.20.0`
		nVersionParts := 3
		versionParts := strings.SplitN(td.Node.GetLibraryInfo().GetExporterVersion(), "-", nVersionParts)
		if md.Service.Language.Name == nil && len(versionParts) == nVersionParts {
			md.Service.Language.Name = &versionParts[1]
		}
		if v := versionParts[len(versionParts)-1]; v != "" {
			md.Service.Agent.Version = &v
		} else {
			jaegerVersion := "unknown"
			md.Service.Agent.Version = &jaegerVersion
		}
		agentName := AgentNameJaeger
		if md.Service.Language.Name != nil {
			agentName = truncate(agentName + "/" + *md.Service.Language.Name)
		}
		md.Service.Agent.Name = &agentName

		if attributes := td.Node.GetAttributes(); attributes != nil {
			if clientUUID, ok := attributes["client-uuid"]; ok {
				clientUUID = truncate(clientUUID)
				md.Service.Agent.EphemeralId = &clientUUID
				delete(td.Node.Attributes, "client-uuid")
			}
			if ip, ok := attributes["ip"]; ok {
				if md.System == nil {
					md.System = &metadata.System{}
				}
				md.System.IP = utility.ParseIP(ip)
				delete(td.Node.Attributes, "ip")
			}
		}
	default:
		agentName := strings.Title(td.SourceFormat)
		md.Service.Agent.Name = &agentName
		version := "unknown"
		md.Service.Agent.Version = &version
	}

	if md.Service.Language.Name == nil {
		unknown := "unknown"
		md.Service.Language.Name = &unknown
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

func parseTransaction(span *tracepb.Span, hostname string, event *model_transaction.Event) {
	labels := make(common.MapStr)
	var http model.Http
	var component string
	var result string
	var hasFailed bool
	var isHTTP bool
	for kDots, v := range span.Attributes.GetAttributeMap() {
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
				intv := int(v.IntValue)
				http.Response = &model.Resp{MinimalResp: model.MinimalResp{StatusCode: &intv}}
				result = statusCodeResult(intv)
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
				event.Url = parseURL(v.StringValue.Value, hostname)
				isHTTP = true
			case "http.status_code":
				if intv, err := strconv.Atoi(v.StringValue.Value); err == nil {
					http.Response = &model.Resp{MinimalResp: model.MinimalResp{StatusCode: &intv}}
					result = statusCodeResult(intv)
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
			case "type":
				event.Type = truncate(v.StringValue.Value)
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
		} else if component != "" {
			event.Type = component
		} else {
			event.Type = "custom"
		}
	}

	if isHTTP {
		if code := int(span.GetStatus().GetCode()); code != 0 {
			result = statusCodeResult(code)
			if http.Response == nil {
				http.Response = &model.Resp{MinimalResp: model.MinimalResp{StatusCode: &code}}
			}
		}
		event.Http = &http
	}

	if result == "" {
		if hasFailed {
			result = "Error"
		} else {
			result = "Success"
		}
	}
	event.Result = &result

	if len(labels) == 0 {
		return
	}
	l := model.Labels(labels)
	event.Labels = &l
}

func parseSpan(span *tracepb.Span, event *model_span.Event) {
	labels := make(common.MapStr)

	var http model_span.HTTP
	var db model_span.DB
	var destination model_span.Destination
	var isDBSpan, isHTTPSpan bool
	var component string
	for kDots, v := range span.Attributes.GetAttributeMap() {
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
				db.Instance = &v.StringValue.Value
				isDBSpan = true
			case "db.type":
				db.Type = &v.StringValue.Value
				isDBSpan = true
			case "db.user":
				db.UserName = &v.StringValue.Value
				isDBSpan = true
			case "peer.address":
				val := truncate(v.StringValue.Value)
				destination.Address = &val
			case "component":
				component = truncate(v.StringValue.Value)
				fallthrough
			default:
				utility.DeepUpdate(labels, k, truncate(v.StringValue.Value))
			}
		}
	}

	if destination != (model_span.Destination{}) {
		event.Destination = &destination
	}

	switch {
	case isHTTPSpan:
		if http.StatusCode == nil {
			if code := int(span.GetStatus().GetCode()); code != 0 {
				http.StatusCode = &code
			}
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
	default:
		event.Type = "custom"
		if component != "" {
			event.Subtype = &component
		}
	}

	if len(labels) == 0 {
		return
	}
	event.Labels = labels
}

func parseErrors(logger *logp.Logger, source string, otelSpan *tracepb.Span) []*model_error.Event {
	var errors []*model_error.Event
	for _, log := range otelSpan.GetTimeEvents().GetTimeEvent() {
		var isError, hasMinimalInfo bool
		var err model_error.Event
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
			err.Log = &model_error.Log{Message: logMessage}
		}
		if exMessage != "" || exType != "" {
			err.Exception = &model_error.Exception{}
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

func addTransactionCtxToErr(transaction model_transaction.Event, err *model_error.Event) {
	err.TransactionId = &transaction.Id
	err.TraceId = &transaction.TraceId
	err.ParentId = &transaction.Id
	err.Http = transaction.Http
	err.Url = transaction.Url
	err.TransactionType = &transaction.Type
}

func addSpanCtxToErr(span model_span.Event, hostname string, err *model_error.Event) {
	err.TransactionId = span.TransactionId
	err.TraceId = &span.TraceId
	err.ParentId = &span.Id
	if span.HTTP != nil {
		err.Http = &model.Http{}
		if span.HTTP.StatusCode != nil {
			err.Http.Response = &model.Resp{MinimalResp: model.MinimalResp{StatusCode: span.HTTP.StatusCode}}
		}
		if span.HTTP.Method != nil {
			err.Http.Request = &model.Req{Method: *span.HTTP.Method}
		}
		if span.HTTP.URL != nil {
			err.Url = parseURL(*span.HTTP.URL, hostname)
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

func parseURL(original, hostname string) *model.Url {
	original = truncate(original)
	url, err := url.Parse(original)
	if err != nil {
		return &model.Url{Original: &original}
	}
	if url.Scheme == "" {
		url.Scheme = "http"
	}
	if url.Host == "" {
		url.Host = hostname
	}
	full := truncate(url.String())
	out := &model.Url{
		Original: &original,
		Scheme:   &url.Scheme,
		Full:     &full,
	}
	if path := truncate(url.Path); path != "" {
		out.Path = &path
	}
	if query := truncate(url.RawQuery); query != "" {
		out.Query = &query
	}
	if fragment := url.Fragment; fragment != "" {
		out.Fragment = &fragment
	}
	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		host = truncate(url.Host)
		port = ""
	}
	if host = truncate(host); host != "" {
		out.Domain = &host
	}
	if port = truncate(port); port != "" {
		if intv, err := strconv.Atoi(port); err == nil {
			out.Port = &intv
		}
	}
	return out
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
