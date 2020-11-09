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

package v2

import (
	"encoding/json"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

var (
	patternAlphaNumericExt    = `^[a-zA-Z0-9 _-]+$`
	patternNoDotAsteriskQuote = `^[^.*"]*$` //do not allow '.' '*' '"'
	patternNoAsteriskQuote    = `^[^*"]*$`  //do not allow '*' '"'

	enumOutcome = []string{"success", "failure", "unknown"}
)

// entry points

// errorRoot requires an error event to be present
type errorRoot struct {
	Error errorEvent `json:"error" validate:"required"`
}

// metadatatRoot requires a metadata event to be present
type metadataRoot struct {
	Metadata metadata `json:"metadata" validate:"required"`
}

// metricsetRoot requires a metricset event to be present
type metricsetRoot struct {
	Metricset metricset `json:"metricset" validate:"required"`
}

// spanRoot requires a span event to be present
type spanRoot struct {
	Span span `json:"span" validate:"required"`
}

// transactionRoot requires a transaction event to be present
type transactionRoot struct {
	Transaction transaction `json:"transaction" validate:"required"`
}

// other structs

type context struct {
	// Custom can contain additional metadata to be stored with the event.
	// The format is unspecified and can be deeply nested objects.
	// The information will not be indexed or searchable in Elasticsearch.
	Custom common.MapStr `json:"custom" validate:"patternKeys=patternNoDotAsteriskQuote"`
	// Experimental information is only processed when APM Server is started
	// in development mode and should only be used by APM agent developers
	// implementing new, unreleased features. The format is unspecified.
	Experimental nullable.Interface `json:"experimental"`
	// Message holds details related to message receiving and publishing
	// if the captured event integrates with a messaging system
	Message contextMessage `json:"message"`
	// Page holds information related to the current page and page referers.
	// It is only sent from RUM agents.
	Page contextPage `json:"page"`
	// Response describes the HTTP response information in case the event was
	// created as a result of an HTTP request.
	Response contextResponse `json:"response"`
	// Request describes the HTTP request information in case the event was
	// created as a result of an HTTP request.
	Request contextRequest `json:"request"`
	// Service related information can be sent per event. Information provided
	// here will override the more generic information retrieved from metadata,
	// missing service fields will still by retrieving the metadata information.
	Service contextService `json:"service"`
	// Tags are a flat mapping of user-defined tags. Allowed value types are
	// string, boolean and number values. Tags are indexed and searchable.
	Tags common.MapStr `json:"tags" validate:"patternKeys=patternNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxLengthVals=1024"`
	// User holds information about the correlated user for this event. If
	// user data are provided here, all user related information from metadata
	// is ignored, otherwise the metadata's user information will be stored
	// with the event.
	User user `json:"user"`
}

type contextMessage struct {
	// Age of the message. If the monitored messaging framework provides a
	// timestamp for the message, agents may use it. Otherwise, the sending
	// agent can add a timestamp in milliseconds since the Unix epoch to the
	// message's metadata to be retrieved by the receiving agent. If a
	// timestamp is not available, agents should omit this field.
	Age contextMessageAge `json:"age"`
	// Body of the received message, similar to an HTTP request body
	Body nullable.String `json:"body"`
	// Headers received with the message, similar to HTTP request headers.
	Headers nullable.HTTPHeader `json:"headers"`
	// Queue holds information about the message queue where the message is received.
	Queue contextMessageQueue `json:"queue"`
}

type contextMessageAge struct {
	// Age of the message in milliseconds. If the monitored messaging
	// framework provides a timestamp for the message, agents may use it.
	// Otherwise, the sending agent can add a timestamp in milliseconds since
	// the Unix epoch to the message's metadata to be retrieved by the
	// receiving agent. If a timestamp is not available, agents should omit
	// this field.
	Milliseconds nullable.Int `json:"ms"`
}

type contextMessageQueue struct {
	// Name holds the name of the message queue where the message is received.
	Name nullable.String `json:"name" validate:"maxLength=1024"`
}

type contextPage struct {
	// Referer holds the URL of the page that 'linked' to the current page.
	Referer nullable.String `json:"referer"`
	// URL of the current page
	URL nullable.String `json:"url"`
}

type contextRequest struct {
	// Body only contais the request bod, not the query string information.
	// It can either be a dictionary (for standard HTTP requests) or a raw
	// request body.
	Body nullable.Interface `json:"body" validate:"inputTypes=string;map[string]interface"`
	// Cookies used by the request, parsed as key-value objects.
	Cookies common.MapStr `json:"cookies"`
	// Env holds environment variable information passed to the monitored service.
	Env common.MapStr `json:"env"`
	// Headers includes any HTTP headers sent by the requester. Cookies will
	// be taken by headers if supplied.
	Headers nullable.HTTPHeader `json:"headers"`
	// HTTPVersion holds information about the used HTTP version.
	HTTPVersion nullable.String `json:"http_version" validate:"maxLength=1024"`
	// Method holds information about the method of the HTTP request.
	Method nullable.String `json:"method" validate:"required,maxLength=1024"`
	// Socket holds information related to the recorded request,
	// such as whether or not data were encrypted and the remote address.
	Socket contextRequestSocket `json:"socket"`
	// URL holds information sucha as the raw URL, scheme, host and path.
	URL contextRequestURL `json:"url"`
}

type contextRequestURL struct {
	// Full, possibly agent-assembled URL of the request,
	// e.g. https://example.com:443/search?q=elasticsearch#top.
	Full nullable.String `json:"full" validate:"maxLength=1024"`
	// Hash of the request URL, e.g. 'top'
	Hash nullable.String `json:"hash" validate:"maxLength=1024"`
	// Hostname information of the request, e.g. 'example.com'."
	Hostname nullable.String `json:"hostname" validate:"maxLength=1024"`
	// Path of the request, e.g. '/search'
	Path nullable.String `json:"pathname" validate:"maxLength=1024"`
	// Port of the request, e.g. '443'. Can be sent as string or int.
	Port nullable.Interface `json:"port" validate:"inputTypes=string;int,targetType=int,maxLength=1024"`
	// Protocol information for the recorded request, e.g. 'https:'.
	Protocol nullable.String `json:"protocol" validate:"maxLength=1024"`
	// Raw unparsed URL of the HTTP request line,
	// e.g https://example.com:443/search?q=elasticsearch. This URL may be
	// absolute or relative. For more details, see
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec5.html#sec5.1.2.
	Raw nullable.String `json:"raw" validate:"maxLength=1024"`
	// Search contains the query string information of the request. It is
	// expected to have values delimited by ampersands.
	Search nullable.String `json:"search" validate:"maxLength=1024"`
}

type contextRequestSocket struct {
	// Encrypted indicates whether a request was sent as TLS/HTTPS request.
	Encrypted nullable.Bool `json:"encrypted"`
	// RemoteAddress holds the network address sending the request. It should
	// be obtained through standard APIs and not be parsed from any headers
	// like 'Forwarded'.
	RemoteAddress nullable.String `json:"remote_address"`
}

type contextResponse struct {
	// DecodedBodySize holds the size of the decoded payload.
	DecodedBodySize nullable.Float64 `json:"decoded_body_size"`
	// EncodedBodySize holds the size of the encoded payload.
	EncodedBodySize nullable.Float64 `json:"encoded_body_size"`
	// Finished indicates whether the response was finished or not.
	Finished nullable.Bool `json:"finished"`
	// Headers holds the http headers sent in the http response.
	Headers nullable.HTTPHeader `json:"headers"`
	// HeadersSent indicates whether http headers were sent.
	HeadersSent nullable.Bool `json:"headers_sent"`
	// StatusCode describes the status code sent in the http response.
	StatusCode nullable.Int `json:"status_code"`
	// TransferSize holds the total size of the payload.
	TransferSize nullable.Float64 `json:"transfer_size"`
}

type contextService struct {
	// Agent holds information about the APM agent capturing the event.
	Agent contextServiceAgent `json:"agent"`
	// Environment in which the monitored service is running,
	// e.g. `production` or `staging`.
	Environment nullable.String `json:"environment" validate:"maxLength=1024"`
	// Framework holds information about the framework used in the
	// monitored service.
	Framework contextServiceFramework `json:"framework"`
	// Language holds information about the programming language of the
	// monitored service.
	Language contextServiceLanguage `json:"language"`
	// Name of the monitored service.
	Name nullable.String `json:"name" validate:"maxLength=1024,pattern=patternAlphaNumericExt"`
	// Node must be a unique meaningful name of the service node.
	Node contextServiceNode `json:"node"`
	// Runtime holds information about the language runtime running the
	// monitored service
	Runtime contextServiceRuntime `json:"runtime"`
	// Version of the monitored service.
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type contextServiceAgent struct {
	// EphemeralID is a free format ID used for metrics correlation by agents
	EphemeralID nullable.String `json:"ephemeral_id" validate:"maxLength=1024"`
	// Name of the APM agent capturing information.
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	// Version of the APM agent capturing information.
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type contextServiceFramework struct {
	// Name of the used framework
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	// Version of the used framework
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type contextServiceLanguage struct {
	// Name of the used programming language
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	// Version of the used programming language
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type contextServiceNode struct {
	// Name of the service node
	Name nullable.String `json:"configured_name" validate:"maxLength=1024"`
}

type contextServiceRuntime struct {
	// Name of the language runtime
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	// Version of the language runtime
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

// errorEvent represents an error or a logged error message,
// captured by an APM agent in a monitored service.
type errorEvent struct {
	// Context holds arbitrary contextual information for the event.
	Context context `json:"context"`
	// Culprit identifies the function call which was the primary perpetrator
	// of this event.
	Culprit nullable.String `json:"culprit" validate:"maxLength=1024"`
	// Exception holds information about the original error.
	Exception errorException `json:"exception"`
	// ID holds the hex encoded 128 random bits ID of the event.
	ID nullable.String `json:"id" validate:"required,maxLength=1024"`
	// Log holds additional information added when the error is logged.
	Log errorLog `json:"log"`
	// ParentID holds the hex encoded 64 random bits ID of the parent
	// transaction or span. It must be present if TraceID or TransactionID
	// are set.
	ParentID nullable.String `json:"parent_id" validate:"requiredIfAny=transaction_id;trace_id,maxLength=1024"`
	// Timestamp holds the recorded time of the event, UTC based and formatted
	// as microseconds since Unix epoch.
	Timestamp nullable.TimeMicrosUnix `json:"timestamp"`
	// TraceID holds the hex encoded 128 random bits ID of the correlated
	// trace. It must be present if TransactionID or ParentID are set.
	TraceID nullable.String `json:"trace_id" validate:"requiredIfAny=transaction_id;parent_id,maxLength=1024"`
	// Transaction holds information about the correlated transaction.
	Transaction errorTransactionRef `json:"transaction"`
	// TransactionID holds the hex encoded 64 random bits ID of the correlated
	// transaction. It must be present if TraceID or ParentID are set.
	TransactionID nullable.String `json:"transaction_id" validate:"maxLength=1024"`
	_             struct{}        `validate:"requiredAnyOf=exception;log"`
}

type errorException struct {
	Attributes common.MapStr      `json:"attributes"`
	Code       nullable.Interface `json:"code" validate:"inputTypes=string;int,maxLength=1024"`
	Cause      []errorException   `json:"cause"`
	Handled    nullable.Bool      `json:"handled"`
	Message    nullable.String    `json:"message"`
	Module     nullable.String    `json:"module" validate:"maxLength=1024"`
	Stacktrace []stacktraceFrame  `json:"stacktrace"`
	Type       nullable.String    `json:"type" validate:"maxLength=1024"`
	_          struct{}           `validate:"requiredAnyOf=message;type"`
}

type errorLog struct {
	Level        nullable.String   `json:"level" validate:"maxLength=1024"`
	LoggerName   nullable.String   `json:"logger_name" validate:"maxLength=1024"`
	Message      nullable.String   `json:"message" validate:"required"`
	ParamMessage nullable.String   `json:"param_message" validate:"maxLength=1024"`
	Stacktrace   []stacktraceFrame `json:"stacktrace"`
}

type errorTransactionRef struct {
	// Sampled indicates whether or not the full information for a transaction
	// is captured. If a transaction is unsampled no spans and less context
	// information will be reported.
	Sampled nullable.Bool `json:"sampled"`
	// Type holds the transaction's type, and can have specific keywords
	// within the service's domain (eg: 'request', 'backgroundjob', etc)
	Type nullable.String `json:"type" validate:"maxLength=1024"`
}

type metadata struct {
	Cloud   metadataCloud   `json:"cloud"`
	Labels  common.MapStr   `json:"labels" validate:"patternKeys=patternNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxLengthVals=1024"`
	Process metadataProcess `json:"process"`
	Service metadataService `json:"service" validate:"required"`
	System  metadataSystem  `json:"system"`
	User    user            `json:"user"`
}

type metadataCloud struct {
	Account          metadataCloudAccount  `json:"account"`
	AvailabilityZone nullable.String       `json:"availability_zone" validate:"maxLength=1024"`
	Instance         metadataCloudInstance `json:"instance"`
	Machine          metadataCloudMachine  `json:"machine"`
	Project          metadataCloudProject  `json:"project"`
	Provider         nullable.String       `json:"provider" validate:"required,maxLength=1024"`
	Region           nullable.String       `json:"region" validate:"maxLength=1024"`
}

type metadataCloudAccount struct {
	ID   nullable.String `json:"id" validate:"maxLength=1024"`
	Name nullable.String `json:"name" validate:"maxLength=1024"`
}

type metadataCloudInstance struct {
	ID   nullable.String `json:"id" validate:"maxLength=1024"`
	Name nullable.String `json:"name" validate:"maxLength=1024"`
}

type metadataCloudMachine struct {
	Type nullable.String `json:"type" validate:"maxLength=1024"`
}

type metadataCloudProject struct {
	ID   nullable.String `json:"id" validate:"maxLength=1024"`
	Name nullable.String `json:"name" validate:"maxLength=1024"`
}

type metadataProcess struct {
	Argv  []string        `json:"argv"`
	Pid   nullable.Int    `json:"pid" validate:"required"`
	Ppid  nullable.Int    `json:"ppid"`
	Title nullable.String `json:"title" validate:"maxLength=1024"`
}

type metadataService struct {
	// Agent holds information about the APM agent capturing the event.
	Agent metadataServiceAgent `json:"agent" validate:"required"`
	// Environment in which the monitored service is running,
	// e.g. `production` or `staging`.
	Environment nullable.String `json:"environment" validate:"maxLength=1024"`
	// Framework holds information about the framework used in the
	// monitored service.
	Framework metadataServiceFramework `json:"framework"`
	// Language holds information about the programming language of the
	// monitored service.
	Language metadataServiceLanguage `json:"language"`
	// Name of the monitored service.
	Name nullable.String `json:"name" validate:"required,minLength=1,maxLength=1024,pattern=patternAlphaNumericExt"`
	// Node must be a unique meaningful name of the service node.
	Node metadataServiceNode `json:"node"`
	// Runtime holds information about the language runtime running the
	// monitored service
	Runtime metadataServiceRuntime `json:"runtime"`
	// Version of the monitored service.
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type metadataServiceAgent struct {
	// EphemeralID is a free format ID used for metrics correlation by agents
	EphemeralID nullable.String `json:"ephemeral_id" validate:"maxLength=1024"`
	// Name of the APM agent capturing information.
	Name nullable.String `json:"name" validate:"required,minLength=1,maxLength=1024"`
	// Version of the APM agent capturing information.
	Version nullable.String `json:"version" validate:"required,maxLength=1024"`
}

type metadataServiceFramework struct {
	// Name of the used framework
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	// Version of the used framework
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type metadataServiceLanguage struct {
	// Name of the used programming language
	Name nullable.String `json:"name" validate:"required,maxLength=1024"`
	// Version of the used programming language
	Version nullable.String `json:"version" validate:"maxLength=1024"`
}

type metadataServiceNode struct {
	// Name of the service node
	Name nullable.String `json:"configured_name" validate:"maxLength=1024"`
}

type metadataServiceRuntime struct {
	// Name of the language runtime
	Name nullable.String `json:"name" validate:"required,maxLength=1024"`
	// Name of the language runtime
	Version nullable.String `json:"version" validate:"required,maxLength=1024"`
}

type metadataSystem struct {
	Architecture       nullable.String          `json:"architecture" validate:"maxLength=1024"`
	ConfiguredHostname nullable.String          `json:"configured_hostname" validate:"maxLength=1024"`
	Container          metadataSystemContainer  `json:"container"`
	DetectedHostname   nullable.String          `json:"detected_hostname" validate:"maxLength=1024"`
	DeprecatedHostname nullable.String          `json:"hostname" validate:"maxLength=1024"`
	Kubernetes         metadataSystemKubernetes `json:"kubernetes"`
	Platform           nullable.String          `json:"platform" validate:"maxLength=1024"`
}

type metadataSystemContainer struct {
	// `id` is the only field in `system.container`,
	// if `system.container:{}` is sent, it should be considered valid
	// if additional attributes are defined in the future, add the required tag
	ID nullable.String `json:"id"` //validate:"required"
}

type metadataSystemKubernetes struct {
	Namespace nullable.String              `json:"namespace" validate:"maxLength=1024"`
	Node      metadataSystemKubernetesNode `json:"node"`
	Pod       metadataSystemKubernetesPod  `json:"pod"`
}

type metadataSystemKubernetesNode struct {
	Name nullable.String `json:"name" validate:"maxLength=1024"`
}

type metadataSystemKubernetesPod struct {
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	UID  nullable.String `json:"uid" validate:"maxLength=1024"`
}

// NOTE(simitt):
// Event is only set by aggregator
// TimeseriesInstanceID is only set by aggregator
type metricset struct {
	// Timestamp holds the recorded time of the event, UTC based and formatted
	// as microseconds since Unix epoch
	Timestamp   nullable.TimeMicrosUnix         `json:"timestamp"`
	Samples     map[string]metricsetSampleValue `json:"samples" validate:"required,patternKeys=patternNoAsteriskQuote"`
	Span        metricsetSpanRef                `json:"span"`
	Tags        common.MapStr                   `json:"tags" validate:"patternKeys=patternNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxLengthVals=1024"`
	Transaction metricsetTransactionRef         `json:"transaction"`
}

// TODO(axw/simitt): add support for ingesting counts/values (histogram metrics)
type metricsetSampleValue struct {
	Value nullable.Float64 `json:"value" validate:"required"`
}

// NOTE(simitt):
// span.DestinationService is only set by aggregator
type metricsetSpanRef struct {
	Subtype nullable.String `json:"subtype" validate:"maxLength=1024"`
	// Type holds the span's type, and can have specific keywords
	// within the service's domain (eg: 'request', 'backgroundjob', etc)
	Type nullable.String `json:"type" validate:"maxLength=1024"`
}

// NOTE(simitt):
// transaction.Result is only set by aggregator and rumV3
// transaction.Root is only set by aggregator
type metricsetTransactionRef struct {
	Name nullable.String `json:"name" validate:"maxLength=1024"`
	// Type holds the transaction's type, and can have specific keywords
	// within the service's domain (eg: 'request', 'backgroundjob', etc)
	Type nullable.String `json:"type" validate:"maxLength=1024"`
}

type span struct {
	Action   nullable.String `json:"action" validate:"maxLength=1024"`
	ChildIDs []string        `json:"child_ids" validate:"maxLength=1024"`
	// Context holds arbitrary contextual information for the event.
	Context    spanContext       `json:"context"`
	Duration   nullable.Float64  `json:"duration" validate:"required,min=0"`
	ID         nullable.String   `json:"id" validate:"required,maxLength=1024"`
	Name       nullable.String   `json:"name" validate:"required,maxLength=1024"`
	Outcome    nullable.String   `json:"outcome" validate:"enum=enumOutcome"`
	ParentID   nullable.String   `json:"parent_id" validate:"required,maxLength=1024"`
	SampleRate nullable.Float64  `json:"sample_rate"`
	Stacktrace []stacktraceFrame `json:"stacktrace"`
	Start      nullable.Float64  `json:"start"`
	Subtype    nullable.String   `json:"subtype" validate:"maxLength=1024"`
	Sync       nullable.Bool     `json:"sync"`
	// Timestamp holds the recorded time of the event, UTC based and formatted
	// as microseconds since Unix epoch
	Timestamp     nullable.TimeMicrosUnix `json:"timestamp"`
	TraceID       nullable.String         `json:"trace_id" validate:"required,maxLength=1024"`
	TransactionID nullable.String         `json:"transaction_id" validate:"maxLength=1024"`
	// Type holds the span's type, and can have specific keywords
	// within the service's domain (eg: 'request', 'backgroundjob', etc)
	Type nullable.String `json:"type" validate:"required,maxLength=1024"`
	_    struct{}        `validate:"requiredAnyOf=start;timestamp"`
}

type spanContext struct {
	Database    spanContextDatabase    `json:"db"`
	Destination spanContextDestination `json:"destination"`
	// Experimental information is only processed when APM Server is started
	// in development mode and should only be used by APM agent developers
	// implementing new, unreleased features. The format is unspecified.
	Experimental nullable.Interface `json:"experimental" `
	HTTP         spanContextHTTP    `json:"http"`
	Message      contextMessage     `json:"message"`
	Service      contextService     `json:"service"`
	Tags         common.MapStr      `json:"tags" validate:"patternKeys=patternNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxLengthVals=1024"`
}

type spanContextDatabase struct {
	Instance     nullable.String `json:"instance"`
	Link         nullable.String `json:"link" validate:"maxLength=1024"`
	RowsAffected nullable.Int    `json:"rows_affected"`
	Statement    nullable.String `json:"statement"`
	Type         nullable.String `json:"type"`
	User         nullable.String `json:"user"`
}

type spanContextDestination struct {
	Address nullable.String               `json:"address" validate:"maxLength=1024"`
	Port    nullable.Int                  `json:"port"`
	Service spanContextDestinationService `json:"service"`
}

type spanContextDestinationService struct {
	Name     nullable.String `json:"name" validate:"required,maxLength=1024"`
	Resource nullable.String `json:"resource" validate:"required,maxLength=1024"`
	Type     nullable.String `json:"type" validate:"required,maxLength=1024"`
}

type spanContextHTTP struct {
	Method     nullable.String         `json:"method" validate:"maxLength=1024"`
	Response   spanContextHTTPResponse `json:"response"`
	StatusCode nullable.Int            `json:"status_code"`
	URL        nullable.String         `json:"url"`
}

type spanContextHTTPResponse struct {
	DecodedBodySize nullable.Float64    `json:"decoded_body_size"`
	EncodedBodySize nullable.Float64    `json:"encoded_body_size"`
	Headers         nullable.HTTPHeader `json:"headers"`
	StatusCode      nullable.Int        `json:"status_code"`
	TransferSize    nullable.Float64    `json:"transfer_size"`
}

type stacktraceFrame struct {
	AbsPath      nullable.String `json:"abs_path"`
	Classname    nullable.String `json:"classname"`
	ColumnNumber nullable.Int    `json:"colno"`
	ContextLine  nullable.String `json:"context_line"`
	Filename     nullable.String `json:"filename"`
	Function     nullable.String `json:"function"`
	LibraryFrame nullable.Bool   `json:"library_frame"`
	LineNumber   nullable.Int    `json:"lineno"`
	Module       nullable.String `json:"module"`
	PostContext  []string        `json:"post_context"`
	PreContext   []string        `json:"pre_context"`
	Vars         common.MapStr   `json:"vars"`
	_            struct{}        `validate:"requiredAnyOf=classname;filename"`
}

type transaction struct {
	// Context holds arbitrary contextual information for the event.
	Context  context          `json:"context"`
	Duration nullable.Float64 `json:"duration" validate:"required,min=0"`
	ID       nullable.String  `json:"id" validate:"required,maxLength=1024"`
	Marks    transactionMarks `json:"marks"`
	Name     nullable.String  `json:"name" validate:"maxLength=1024"`
	Outcome  nullable.String  `json:"outcome" validate:"enum=enumOutcome"`
	ParentID nullable.String  `json:"parent_id" validate:"maxLength=1024"`
	Result   nullable.String  `json:"result" validate:"maxLength=1024"`
	// Sampled indicates whether or not the full information for a transaction
	// is captured. If a transaction is unsampled no spans and less context
	// information will be reported.
	Sampled    nullable.Bool        `json:"sampled"`
	SampleRate nullable.Float64     `json:"sample_rate"`
	SpanCount  transactionSpanCount `json:"span_count" validate:"required"`
	// Timestamp holds the recorded time of the event, UTC based and formatted
	// as microseconds since Unix epoch
	Timestamp nullable.TimeMicrosUnix `json:"timestamp"`
	TraceID   nullable.String         `json:"trace_id" validate:"required,maxLength=1024"`
	// Type holds the transaction's type, and can have specific keywords
	// within the service's domain (eg: 'request', 'backgroundjob', etc)
	Type           nullable.String           `json:"type" validate:"required,maxLength=1024"`
	UserExperience transactionUserExperience `json:"experience"`
}

type transactionMarks struct {
	Events map[string]transactionMarkEvents `json:"-" validate:"patternKeys=patternNoDotAsteriskQuote"`
}

func (m *transactionMarks) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.Events)
}

type transactionMarkEvents struct {
	Measurements map[string]float64 `json:"-" validate:"patternKeys=patternNoDotAsteriskQuote"`
}

func (m *transactionMarkEvents) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.Measurements)
}

type transactionSpanCount struct {
	Dropped nullable.Int `json:"dropped"`
	Started nullable.Int `json:"started" validate:"required"`
}

// transactionUserExperience holds real user (browser) experience metrics.
type transactionUserExperience struct {
	// CumulativeLayoutShift holds the Cumulative Layout Shift (CLS) metric value,
	// or a negative value if CLS is unknown. See https://web.dev/cls/
	CumulativeLayoutShift nullable.Float64 `json:"cls" validate:"min=0"`

	// FirstInputDelay holds the First Input Delay (FID) metric value,
	// or a negative value if FID is unknown. See https://web.dev/fid/
	FirstInputDelay nullable.Float64 `json:"fid" validate:"min=0"`

	// Longtask holds longtask duration/count metrics.
	Longtask longtaskMetrics `json:"longtask"`

	// TotalBlockingTime holds the Total Blocking Time (TBT) metric value,
	// or a negative value if TBT is unknown. See https://web.dev/tbt/
	TotalBlockingTime nullable.Float64 `json:"tbt" validate:"min=0"`
}

type longtaskMetrics struct {
	Count nullable.Int     `json:"count" validate:"required,min=0"`
	Max   nullable.Float64 `json:"max" validate:"required,min=0"`
	Sum   nullable.Float64 `json:"sum" validate:"required,min=0"`
}

type user struct {
	ID    nullable.Interface `json:"id" validate:"maxLength=1024,inputTypes=string;int"`
	Email nullable.String    `json:"email" validate:"maxLength=1024"`
	Name  nullable.String    `json:"username" validate:"maxLength=1024"`
}
