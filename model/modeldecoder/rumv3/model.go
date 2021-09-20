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

package rumv3

import (
	"encoding/json"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

var (
	patternAlphaNumericExt = `^[a-zA-Z0-9 _-]+$`

	enumOutcome = []string{"success", "failure", "unknown"}
)

// entry points

// errorRoot requires an error event to be present
type errorRoot struct {
	Error errorEvent `json:"e" validate:"required"`
}

// metadatatRoot requires a metadata event to be present
type metadataRoot struct {
	Metadata metadata `json:"m" validate:"required"`
}

// transactionRoot requires a transaction event to be present
type transactionRoot struct {
	Transaction transaction `json:"x" validate:"required"`
}

// other structs

type context struct {
	// Custom can contain additional metadata to be stored with the event.
	// The format is unspecified and can be deeply nested objects.
	// The information will not be indexed or searchable in Elasticsearch.
	Custom common.MapStr `json:"cu"`
	// Page holds information related to the current page and page referers.
	// It is only sent from RUM agents.
	Page contextPage `json:"p"`
	// Response describes the HTTP response information in case the event was
	// created as a result of an HTTP request.
	Response contextResponse `json:"r"`
	// Request describes the HTTP request information in case the event was
	// created as a result of an HTTP request.
	Request contextRequest `json:"q"`
	// Service related information can be sent per event. Information provided
	// here will override the more generic information retrieved from metadata,
	// missing service fields will be retrieved from the metadata information.
	Service contextService `json:"se"`
	// Tags are a flat mapping of user-defined tags. Allowed value types are
	// string, boolean and number values. Tags are indexed and searchable.
	Tags common.MapStr `json:"g" validate:"inputTypesVals=string;bool;number,maxLengthVals=1024"`
	// User holds information about the correlated user for this event. If
	// user data are provided here, all user related information from metadata
	// is ignored, otherwise the metadata's user information will be stored
	// with the event.
	User user `json:"u"`
}

type contextPage struct {
	// Referer holds the URL of the page that 'linked' to the current page.
	Referer nullable.String `json:"rf"`
	// URL of the current page
	URL nullable.String `json:"url"`
}

type contextRequest struct {
	// Env holds environment variable information passed to the monitored service.
	Env common.MapStr `json:"en"`
	// Headers includes any HTTP headers sent by the requester. Cookies will
	// be taken by headers if supplied.
	Headers nullable.HTTPHeader `json:"he"`
	// HTTPVersion holds information about the used HTTP version.
	HTTPVersion nullable.String `json:"hve" validate:"maxLength=1024"`
	// Method holds information about the method of the HTTP request.
	Method nullable.String `json:"mt" validate:"required,maxLength=1024"`
}

type contextResponse struct {
	// DecodedBodySize holds the size of the decoded payload.
	DecodedBodySize nullable.Float64 `json:"dbs"`
	// EncodedBodySize holds the size of the encoded payload.
	EncodedBodySize nullable.Float64 `json:"ebs"`
	// Headers holds the http headers sent in the http response.
	Headers nullable.HTTPHeader `json:"he"`
	// StatusCode sent in the http response.
	StatusCode nullable.Int `json:"sc"`
	// TransferSize holds the total size of the payload.
	TransferSize nullable.Float64 `json:"ts"`
}

type contextService struct {
	// Agent holds information about the APM agent capturing the event.
	Agent contextServiceAgent `json:"a"`
	// Environment in which the monitored service is running,
	// e.g. `production` or `staging`.
	Environment nullable.String `json:"en" validate:"maxLength=1024"`
	// Framework holds information about the framework used in the
	// monitored service.
	Framework contextServiceFramework `json:"fw"`
	// Language holds information about the programming language of the
	// monitored service.
	Language contextServiceLanguage `json:"la"`
	// Name of the monitored service.
	Name nullable.String `json:"n" validate:"maxLength=1024,pattern=patternAlphaNumericExt"`
	// Runtime holds information about the language runtime running the
	// monitored service
	Runtime contextServiceRuntime `json:"ru"`
	// Version of the monitored service.
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type contextServiceAgent struct {
	// Name of the APM agent capturing information.
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Version of the APM agent capturing information.
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type contextServiceFramework struct {
	// Name of the used framework
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Version of the used framework
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type contextServiceLanguage struct {
	// Name of the used programming language
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Version of the used programming language
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type contextServiceRuntime struct {
	// Name of the language runtime
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Version of the language runtime
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type errorEvent struct {
	// Context holds arbitrary contextual information for the event.
	Context context `json:"c"`
	// Culprit identifies the function call which was the primary perpetrator
	// of this event.
	Culprit nullable.String `json:"cl" validate:"maxLength=1024"`
	// Exception holds information about the original error.
	// The information is language specific.
	Exception errorException `json:"ex"`
	// ID holds the hex encoded 128 random bits ID of the event.
	ID nullable.String `json:"id" validate:"required,maxLength=1024"`
	// Log holds additional information added when the error is logged.
	Log errorLog `json:"log"`
	// ParentID holds the hex encoded 64 random bits ID of the parent
	// transaction or span.
	ParentID nullable.String `json:"pid" validate:"requiredIfAny=xid;tid,maxLength=1024"`
	// Timestamp holds the recorded time of the event, UTC based and formatted
	// as microseconds since Unix epoch.
	Timestamp nullable.TimeMicrosUnix `json:"timestamp"`
	// TraceID holds the hex encoded 128 random bits ID of the correlated trace.
	TraceID nullable.String `json:"tid" validate:"requiredIfAny=xid;pid,maxLength=1024"`
	// Transaction holds information about the correlated transaction.
	Transaction errorTransactionRef `json:"x"`
	// TransactionID holds the hex encoded 64 random bits ID of the correlated
	// transaction.
	TransactionID nullable.String `json:"xid" validate:"maxLength=1024"`
	_             struct{}        `validate:"requiredAnyOf=ex;log"`
}

type errorException struct {
	// Attributes of the exception.
	Attributes common.MapStr `json:"at"`
	// Code that is set when the error happened, e.g. database error code.
	Code nullable.Interface `json:"cd" validate:"inputTypes=string;int,maxLength=1024"`
	// Cause can hold a collection of error exceptions representing chained
	// exceptions. The chain starts with the outermost exception, followed
	// by its cause, and so on.
	Cause []errorException `json:"ca"`
	// Handled indicates whether the error was caught in the code or not.
	Handled nullable.Bool `json:"hd"`
	// Message contains the originally captured error message.
	Message nullable.String `json:"mg"`
	// Module describes the exception type's module namespace.
	Module nullable.String `json:"mo" validate:"maxLength=1024"`
	// Stacktrace information of the captured exception.
	Stacktrace []stacktraceFrame `json:"st"`
	// Type of the exception.
	Type nullable.String `json:"t" validate:"maxLength=1024"`
	_    struct{}        `validate:"requiredAnyOf=mg;t"`
}

type errorLog struct {
	// Level represents the severity of the recorded log.
	Level nullable.String `json:"lv" validate:"maxLength=1024"`
	// LoggerName holds the name of the used logger instance.
	LoggerName nullable.String `json:"ln" validate:"maxLength=1024"`
	// Message of the logged error. In case a parameterized message is captured,
	// Message should contain the same information, but with any placeholders
	// being replaced.
	Message nullable.String `json:"mg" validate:"required"`
	// ParamMessage should contain the same information as Message, but with
	// placeholders where parameters were logged, e.g. 'error connecting to %s'.
	// The string is not interpreted, allowing differnt placeholders per client
	// languange. The information might be used to group errors together.
	ParamMessage nullable.String `json:"pmg" validate:"maxLength=1024"`
	// Stacktrace information of the captured error.
	Stacktrace []stacktraceFrame `json:"st"`
}

type errorTransactionRef struct {
	// Sampled indicates whether or not the full information for a transaction
	// is captured. If a transaction is unsampled no spans and less context
	// information will be reported.
	Sampled nullable.Bool `json:"sm"`
	// Type expresses the correlated transaction's type as keyword that has
	// specific relevance within the service's domain,
	// eg: 'request', 'backgroundjob'.
	Type nullable.String `json:"t" validate:"maxLength=1024"`
}

type metadata struct {
	// Labels are a flat mapping of user-defined tags. Allowed value types are
	// string, boolean and number values. Labels are indexed and searchable.
	Labels common.MapStr `json:"l" validate:"inputTypesVals=string;bool;number,maxLengthVals=1024"`
	// Service metadata about the monitored service.
	Service metadataService `json:"se" validate:"required"`
	// User metadata, which can be overwritten on a per event basis.
	User user `json:"u"`
	// Network holds information about the network over which the
	// monitored service is communicating.
	Network network `json:"n"`
}

type metadataService struct {
	// Agent holds information about the APM agent capturing the event.
	Agent metadataServiceAgent `json:"a" validate:"required"`
	// Environment in which the monitored service is running,
	// e.g. `production` or `staging`.
	Environment nullable.String `json:"en" validate:"maxLength=1024"`
	// Framework holds information about the framework used in the
	// monitored service.
	Framework metadataServiceFramework `json:"fw"`
	// Language holds information about the programming language of the
	// monitored service.
	Language metadataServiceLanguage `json:"la"`
	// Name of the monitored service.
	Name nullable.String `json:"n" validate:"required,minLength=1,maxLength=1024,pattern=patternAlphaNumericExt"`
	// Runtime holds information about the language runtime running the
	// monitored service
	Runtime metadataServiceRuntime `json:"ru"`
	// Version of the monitored service.
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type metadataServiceAgent struct {
	// Name of the APM agent capturing information.
	Name nullable.String `json:"n" validate:"required,minLength=1,maxLength=1024"`
	// Version of the APM agent capturing information.
	Version nullable.String `json:"ve" validate:"required,maxLength=1024"`
}

type metadataServiceFramework struct {
	// Name of the used framework
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Version of the used framework
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type metadataServiceLanguage struct {
	// Name of the used programming language
	Name nullable.String `json:"n" validate:"required,maxLength=1024"`
	// Version of the used programming language
	Version nullable.String `json:"ve" validate:"maxLength=1024"`
}

type metadataServiceRuntime struct {
	// Name of the language runtime
	Name nullable.String `json:"n" validate:"required,maxLength=1024"`
	// Name of the language runtime
	Version nullable.String `json:"ve" validate:"required,maxLength=1024"`
}

type network struct {
	Connection networkConnection `json:"c"`
}

type networkConnection struct {
	Type nullable.String `json:"t" validate:"maxLength=1024"`
}

type transactionMetricset struct {
	// Samples hold application metrics collected from the agent.
	Samples transactionMetricsetSamples `json:"sa" validate:"required"`
	// Span holds selected information about the correlated transaction.
	Span metricsetSpanRef `json:"y"`
}

type transactionMetricsetSamples struct {
	// TransactionBreakdownCount The number of transactions for which breakdown metrics (span.self_time) have been created.
	TransactionBreakdownCount metricsetSampleValue `json:"xbc"`
	// SpanSelfTimeCount holds the count of the related spans' self_time.
	SpanSelfTimeCount metricsetSampleValue `json:"ysc"`
	// SpanSelfTimeSum holds the sum of the related spans' self_time.
	SpanSelfTimeSum metricsetSampleValue `json:"yss"`
}

type metricsetSampleValue struct {
	// Value holds the value of a single metric sample.
	Value nullable.Float64 `json:"v" validate:"required"`
}

type metricsetSpanRef struct {
	// Subtype is a further sub-division of the type (e.g. postgresql, elasticsearch)
	Subtype nullable.String `json:"su" validate:"maxLength=1024"`
	// Type expresses the correlated span's type as keyword that has specific
	// relevance within the service's domain, eg: 'request', 'backgroundjob'.
	Type nullable.String `json:"t" validate:"maxLength=1024"`
}

type span struct {
	// Action holds the specific kind of event within the sub-type represented
	// by the span (e.g. query, connect)
	Action nullable.String `json:"ac" validate:"maxLength=1024"`
	// Context holds arbitrary contextual information for the event.
	Context spanContext `json:"c"`
	// Duration of the span in milliseconds
	Duration nullable.Float64 `json:"d" validate:"required,min=0"`
	// ID holds the hex encoded 64 random bits ID of the event.
	ID nullable.String `json:"id" validate:"required,maxLength=1024"`
	// Name is the generic designation of a span in the scope of a transaction.
	Name nullable.String `json:"n" validate:"required,maxLength=1024"`
	// Outcome of the span: success, failure, or unknown. Outcome may be one of
	// a limited set of permitted values describing the success or failure of
	// the span. It can be used for calculating error rates for outgoing requests.
	Outcome nullable.String `json:"o" validate:"enum=enumOutcome"`
	// ParentIndex is the index of the parent span in the list. Absent when
	// the parent is a transaction.
	ParentIndex nullable.Int `json:"pi"`
	// SampleRate applied to the monitored service at the time where this span
	// was recorded.
	SampleRate nullable.Float64 `json:"sr"`
	// Stacktrace connected to this span event.
	Stacktrace []stacktraceFrame `json:"st"`
	// Start is the offset relative to the transaction's timestamp identifying
	// the start of the span, in milliseconds.
	Start nullable.Float64 `json:"s" validate:"required"`
	// Subtype is a further sub-division of the type (e.g. postgresql, elasticsearch)
	Subtype nullable.String `json:"su" validate:"maxLength=1024"`
	// Sync indicates whether the span was executed synchronously or asynchronously.
	Sync nullable.Bool `json:"sy"`
	// Type holds the span's type, and can have specific keywords
	// within the service's domain (eg: 'request', 'backgroundjob', etc)
	Type nullable.String `json:"t" validate:"required,maxLength=1024"`
}

type spanContext struct {
	// Destination contains contextual data about the destination of spans
	Destination spanContextDestination `json:"dt"`
	// HTTP contains contextual information when the span concerns an HTTP request.
	HTTP spanContextHTTP `json:"h"`
	// Service related information can be sent per span. Information provided
	// here will override the more generic information retrieved from metadata,
	// missing service fields will be retrieved from the metadata information.
	Service spanContextService `json:"se"`
	// Tags are a flat mapping of user-defined tags. Allowed value types are
	// string, boolean and number values. Tags are indexed and searchable.
	Tags common.MapStr `json:"g" validate:"inputTypesVals=string;bool;number,maxLengthVals=1024"`
}

type spanContextDestination struct {
	// Address is the destination network address:
	// hostname (e.g. 'localhost'),
	// FQDN (e.g. 'elastic.co'),
	// IPv4 (e.g. '127.0.0.1')
	// IPv6 (e.g. '::1')
	Address nullable.String `json:"ad" validate:"maxLength=1024"`
	// Port is the destination network port (e.g. 443)
	Port nullable.Int `json:"po"`
	// Service describes the destination service
	Service spanContextDestinationService `json:"se"`
}

type spanContextDestinationService struct {
	// Name is the identifier for the destination service,
	// e.g. 'http://elastic.co', 'elasticsearch', 'rabbitmq'
	// DEPRECATED: this field will be removed in a future release
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Resource identifies the destination service resource being operated on
	// e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name'
	Resource nullable.String `json:"rc" validate:"required,maxLength=1024"`
	// Type of the destination service, e.g. db, elasticsearch. Should
	// typically be the same as span.type.
	// DEPRECATED: this field will be removed in a future release
	Type nullable.String `json:"t" validate:"maxLength=1024"`
}

type spanContextHTTP struct {
	// Method holds information about the method of the HTTP request.
	Method nullable.String `json:"mt" validate:"maxLength=1024"`
	// Response describes the HTTP response information in case the event was
	// created as a result of an HTTP request.
	Response spanContextHTTPResponse `json:"r"`
	// Deprecated: Use Response.StatusCode instead.
	// StatusCode sent in the http response.
	StatusCode nullable.Int `json:"sc"`
	// URL is the raw url of the correlating HTTP request.
	URL nullable.String `json:"url"`
}

type spanContextHTTPResponse struct {
	// DecodedBodySize holds the size of the decoded payload.
	DecodedBodySize nullable.Float64 `json:"dbs"`
	// EncodedBodySize holds the size of the encoded payload.
	EncodedBodySize nullable.Float64 `json:"ebs"`
	// TransferSize holds the total size of the payload.
	TransferSize nullable.Float64 `json:"ts"`
}

type spanContextService struct {
	// Agent holds information about the APM agent capturing the event.
	Agent contextServiceAgent `json:"a"`
	// Name of the monitored service.
	Name nullable.String `json:"n" validate:"maxLength=1024,pattern=patternAlphaNumericExt"`
}

type stacktraceFrame struct {
	// AbsPath is the absolute path of the frame's file.
	AbsPath nullable.String `json:"ap"`
	// Classname of the frame.
	Classname nullable.String `json:"cn"`
	// ColumnNumber of the frame.
	ColumnNumber nullable.Int `json:"co"`
	// ContextLine is the line from the frame's file.
	ContextLine nullable.String `json:"cli"`
	// Filename is the relative name of the frame's file.
	Filename nullable.String `json:"f" validate:"required"`
	// Function represented by the frame.
	Function nullable.String `json:"fn"`
	// LineNumber of the frame.
	LineNumber nullable.Int `json:"li"`
	// Module to which the frame belongs to.
	Module nullable.String `json:"mo"`
	// PostContext is a slice of code lines immediately before the line
	// from the frame's file.
	PostContext []string `json:"poc"`
	// PreContext is a slice of code lines immediately after the line
	// from the frame's file.
	PreContext []string `json:"prc"`
}

type transaction struct {
	// Context holds arbitrary contextual information for the event.
	Context context `json:"c"`
	// Duration how long the transaction took to complete, in milliseconds
	// with 3 decimal points.
	Duration nullable.Float64 `json:"d" validate:"required,min=0"`
	// ID holds the hex encoded 64 random bits ID of the event.
	ID nullable.String `json:"id" validate:"required,maxLength=1024"`
	// Marks capture the timing of a significant event during the lifetime of
	// a transaction. Marks are organized into groups and can be set by the
	// user or the agent. Marks are only reported by RUM agents.
	Marks transactionMarks `json:"k"`
	// Metricsets is a collection metrics related to this transaction.
	Metricsets []transactionMetricset `json:"me"`
	// Name is the generic designation of a transaction in the scope of a
	// single service, eg: 'GET /users/:id'.
	Name nullable.String `json:"n" validate:"maxLength=1024"`
	// Outcome of the transaction with a limited set of permitted values,
	// describing the success or failure of the transaction from the service's
	// perspective. It is used for calculating error rates for incoming requests.
	// Permitted values: success, failure, unknown.
	Outcome nullable.String `json:"o" validate:"enum=enumOutcome"`
	// ParentID holds the hex encoded 64 random bits ID of the parent
	// transaction or span.
	ParentID nullable.String `json:"pid" validate:"maxLength=1024"`
	// Result of the transaction. For HTTP-related transactions, this should
	// be the status code formatted like 'HTTP 2xx'.
	Result nullable.String `json:"rt" validate:"maxLength=1024"`
	// Sampled indicates whether or not the full information for a transaction
	// is captured. If a transaction is unsampled no spans and less context
	// information will be reported.
	Sampled nullable.Bool `json:"sm"`
	// SampleRate applied to the monitored service at the time where this transaction
	// was recorded. Allowed values are [0..1]. A SampleRate <1 indicates that
	// not all spans are recorded.
	SampleRate nullable.Float64 `json:"sr"`
	// Session holds optional transaction session information for RUM.
	Session transactionSession `json:"ses"`
	// SpanCount counts correlated spans.
	SpanCount transactionSpanCount `json:"yc" validate:"required"`
	// Spans is a collection of spans related to this transaction.
	Spans []span `json:"y"`
	// TraceID holds the hex encoded 128 random bits ID of the correlated trace.
	TraceID nullable.String `json:"tid" validate:"required,maxLength=1024"`
	// Type expresses the transaction's type as keyword that has specific
	// relevance within the service's domain, eg: 'request', 'backgroundjob'.
	Type nullable.String `json:"t" validate:"required,maxLength=1024"`
	// UserExperience holds metrics for measuring real user experience.
	// This information is only sent by RUM agents.
	UserExperience transactionUserExperience `json:"exp"`
}

type transactionSession struct {
	// ID holds a session ID for grouping a set of related transactions.
	ID nullable.String `json:"id" validate:"required"`

	// Sequence holds an optional sequence number for a transaction within
	// a session. It is not meaningful to compare sequences across two
	// different sessions.
	Sequence nullable.Int `json:"seq" validate:"min=1"`
}

type transactionMarks struct {
	Events map[string]transactionMarkEvents `json:"-"`
}

var markEventsLongNames = map[string]string{
	"a":  "agent",
	"nt": "navigationTiming",
}

func (m *transactionMarks) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &m.Events); err != nil {
		return err
	}
	for name, val := range m.Events {
		nameLong, ok := markEventsLongNames[name]
		if !ok {
			// there is no long name defined for this event
			continue
		}
		delete(m.Events, name)
		m.Events[nameLong] = val
	}
	return nil
}

type transactionMarkEvents struct {
	Measurements map[string]float64 `json:"-"`
}

func (m *transactionMarkEvents) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &m.Measurements); err != nil {
		return err
	}
	for name, val := range m.Measurements {
		nameLong, ok := markMeasurementsLongNames[name]
		if !ok {
			// there is no long name defined for this measurement
			continue
		}
		delete(m.Measurements, name)
		m.Measurements[nameLong] = val
	}
	return nil
}

var markMeasurementsLongNames = map[string]string{
	"ce": "connectEnd",
	"cs": "connectStart",
	"dc": "domComplete",
	"de": "domContentLoadedEventEnd",
	"di": "domInteractive",
	"dl": "domLoading",
	"ds": "domContentLoadedEventStart",
	"ee": "loadEventEnd",
	"es": "loadEventStart",
	"fb": "timeToFirstByte",
	"fp": "firstContentfulPaint",
	"fs": "fetchStart",
	"le": "domainLookupEnd",
	"lp": "largestContentfulPaint",
	"ls": "domainLookupStart",
	"re": "responseEnd",
	"rs": "responseStart",
	"qs": "requestStart",
}

type transactionSpanCount struct {
	// Dropped is the number of correlated spans that have been dropped by
	// the APM agent recording the transaction.
	Dropped nullable.Int `json:"dd"`
	// Started is the number of correlated spans that are recorded.
	Started nullable.Int `json:"sd" validate:"required"`
}

// userExperience holds real user (browser) experience metrics.
type transactionUserExperience struct {
	// CumulativeLayoutShift holds the Cumulative Layout Shift (CLS) metric value,
	// or a negative value if CLS is unknown. See https://web.dev/cls/
	CumulativeLayoutShift nullable.Float64 `json:"cls" validate:"min=0"`
	// FirstInputDelay holds the First Input Delay (FID) metric value,
	// or a negative value if FID is unknown. See https://web.dev/fid/
	FirstInputDelay nullable.Float64 `json:"fid" validate:"min=0"`
	// TotalBlockingTime holds the Total Blocking Time (TBT) metric value,
	// or a negative value if TBT is unknown. See https://web.dev/tbt/
	TotalBlockingTime nullable.Float64 `json:"tbt" validate:"min=0"`
	// Longtask holds longtask duration/count metrics.
	Longtask longtaskMetrics `json:"lt"`
}

type longtaskMetrics struct {
	// Count is the total number of of longtasks.
	Count nullable.Int `json:"count" validate:"required,min=0"`
	// Max longtask duration
	Max nullable.Float64 `json:"max" validate:"required,min=0"`
	// Sum of longtask durations
	Sum nullable.Float64 `json:"sum" validate:"required,min=0"`
}

type user struct {
	// Domain of the user
	Domain nullable.String `json:"ud" validate:"maxLength=1024"`
	// ID identifies the logged in user, e.g. can be the primary key of the user
	ID nullable.Interface `json:"id" validate:"maxLength=1024,inputTypes=string;int"`
	// Email of the user.
	Email nullable.String `json:"em" validate:"maxLength=1024"`
	// Name of the user.
	Name nullable.String `json:"un" validate:"maxLength=1024"`
}
