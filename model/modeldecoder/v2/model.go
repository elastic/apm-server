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
	"regexp"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

var (
	regexpAlphaNumericExt    = regexp.MustCompile("^[a-zA-Z0-9 _-]+$")
	regexpNoDotAsteriskQuote = regexp.MustCompile("^[^.*\"]*$") //do not allow '.' '*' '"'
	regexpNoAsteriskQuote    = regexp.MustCompile("^[^*\"]*$")  //do not allow '*' '"'

	enumOutcome = []string{"success", "failure", "unknown"}
)

// entry points

type errorRoot struct {
	Error errorEvent `json:"error" validate:"required"`
}

type metadataRoot struct {
	Metadata metadata `json:"metadata" validate:"required"`
}

type metricsetRoot struct {
	Metricset metricset `json:"metricset" validate:"required"`
}

type spanRoot struct {
	Span span `json:"span" validate:"required"`
}

type transactionRoot struct {
	Transaction transaction `json:"transaction" validate:"required"`
}

// other structs

type context struct {
	Custom       common.MapStr      `json:"custom" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Experimental nullable.Interface `json:"experimental"`
	Message      contextMessage     `json:"message"`
	Page         contextPage        `json:"page"`
	Response     contextResponse    `json:"response"`
	Request      contextRequest     `json:"request"`
	Service      contextService     `json:"service"`
	Tags         common.MapStr      `json:"tags" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
	User         user               `json:"user"`
}

type contextMessage struct {
	Age     contextMessageAge   `json:"age"`
	Body    nullable.String     `json:"body"`
	Headers nullable.HTTPHeader `json:"headers"`
	Queue   contextMessageQueue `json:"queue"`
}

type contextMessageAge struct {
	Milliseconds nullable.Int `json:"ms"`
}

type contextMessageQueue struct {
	Name nullable.String `json:"name" validate:"max=1024"`
}

type contextPage struct {
	Referer nullable.String `json:"referer"`
	URL     nullable.String `json:"url"`
}

type contextRequest struct {
	Body        nullable.Interface   `json:"body" validate:"inputTypes=string;map[string]interface"`
	Cookies     common.MapStr        `json:"cookies"`
	Env         common.MapStr        `json:"env"`
	Headers     nullable.HTTPHeader  `json:"headers"`
	HTTPVersion nullable.String      `json:"http_version" validate:"max=1024"`
	Method      nullable.String      `json:"method" validate:"required,max=1024"`
	Socket      contextRequestSocket `json:"socket"`
	// context.request.url was required in json schema,
	// but none of its attributes is required, which could lead to
	// an empty URL struct - no difference to making it optional
	URL contextRequestURL `json:"url"`
}

type contextRequestURL struct {
	Full     nullable.String    `json:"full" validate:"max=1024"`
	Hash     nullable.String    `json:"hash" validate:"max=1024"`
	Hostname nullable.String    `json:"hostname" validate:"max=1024"`
	Path     nullable.String    `json:"pathname" validate:"max=1024"`
	Port     nullable.Interface `json:"port" validate:"inputTypes=string;int,targetType=int"`
	Protocol nullable.String    `json:"protocol" validate:"max=1024"`
	Raw      nullable.String    `json:"raw" validate:"max=1024"`
	Search   nullable.String    `json:"search" validate:"max=1024"`
}

type contextRequestSocket struct {
	Encrypted     nullable.Bool   `json:"encrypted"`
	RemoteAddress nullable.String `json:"remote_address"`
}

type contextResponse struct {
	DecodedBodySize nullable.Float64    `json:"decoded_body_size"`
	EncodedBodySize nullable.Float64    `json:"encoded_body_size"`
	Finished        nullable.Bool       `json:"finished"`
	Headers         nullable.HTTPHeader `json:"headers"`
	HeadersSent     nullable.Bool       `json:"headers_sent"`
	StatusCode      nullable.Int        `json:"status_code"`
	TransferSize    nullable.Float64    `json:"transfer_size"`
}

type contextService struct {
	Agent       contextServiceAgent     `json:"agent"`
	Environment nullable.String         `json:"environment" validate:"max=1024"`
	Framework   contextServiceFramework `json:"framework"`
	Language    contextServiceLanguage  `json:"language"`
	Name        nullable.String         `json:"name" validate:"max=1024,pattern=regexpAlphaNumericExt"`
	Node        contextServiceNode      `json:"node"`
	Runtime     contextServiceRuntime   `json:"runtime"`
	Version     nullable.String         `json:"version" validate:"max=1024"`
}

type contextServiceAgent struct {
	EphemeralID nullable.String `json:"ephemeral_id" validate:"max=1024"`
	Name        nullable.String `json:"name" validate:"max=1024"`
	Version     nullable.String `json:"version" validate:"max=1024"`
}

type contextServiceFramework struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type contextServiceLanguage struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type contextServiceNode struct {
	Name nullable.String `json:"configured_name" validate:"max=1024"`
}

type contextServiceRuntime struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type errorEvent struct {
	Context       context                 `json:"context"`
	Culprit       nullable.String         `json:"culprit" validate:"max=1024"`
	Exception     errorException          `json:"exception"`
	ID            nullable.String         `json:"id" validate:"required,max=1024"`
	Log           errorLog                `json:"log"`
	ParentID      nullable.String         `json:"parent_id" validate:"requiredIfAny=transaction_id;trace_id,max=1024"`
	Timestamp     nullable.TimeMicrosUnix `json:"timestamp"`
	TraceID       nullable.String         `json:"trace_id" validate:"requiredIfAny=transaction_id;parent_id,max=1024"`
	Transaction   errorTransactionRef     `json:"transaction"`
	TransactionID nullable.String         `json:"transaction_id" validate:"max=1024"`
	_             struct{}                `validate:"requiredOneOf=exception;log"`
}

type errorException struct {
	Attributes common.MapStr      `json:"attributes"`
	Code       nullable.Interface `json:"code" validate:"inputTypes=string;int,max=1024"`
	Cause      []errorException   `json:"cause"`
	Handled    nullable.Bool      `json:"handled"`
	Message    nullable.String    `json:"message"`
	Module     nullable.String    `json:"module" validate:"max=1024"`
	Stacktrace []stacktraceFrame  `json:"stacktrace"`
	Type       nullable.String    `json:"type" validate:"max=1024"`
	_          struct{}           `validate:"requiredOneOf=message;type"`
}

type errorLog struct {
	Level        nullable.String   `json:"level" validate:"max=1024"`
	LoggerName   nullable.String   `json:"logger_name" validate:"max=1024"`
	Message      nullable.String   `json:"message" validate:"required"`
	ParamMessage nullable.String   `json:"param_message" validate:"max=1024"`
	Stacktrace   []stacktraceFrame `json:"stacktrace"`
}

type errorTransactionRef struct {
	Sampled nullable.Bool   `json:"sampled"`
	Type    nullable.String `json:"type" validate:"max=1024"`
}

type metadata struct {
	Cloud   metadataCloud   `json:"cloud"`
	Labels  common.MapStr   `json:"labels" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
	Process metadataProcess `json:"process"`
	Service metadataService `json:"service" validate:"required"`
	System  metadataSystem  `json:"system"`
	User    user            `json:"user"`
}

type metadataCloud struct {
	Account          metadataCloudAccount  `json:"account"`
	AvailabilityZone nullable.String       `json:"availability_zone" validate:"max=1024"`
	Instance         metadataCloudInstance `json:"instance"`
	Machine          metadataCloudMachine  `json:"machine"`
	Project          metadataCloudProject  `json:"project"`
	Provider         nullable.String       `json:"provider" validate:"required,max=1024"`
	Region           nullable.String       `json:"region" validate:"max=1024"`
}

type metadataCloudAccount struct {
	ID   nullable.String `json:"id" validate:"max=1024"`
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataCloudInstance struct {
	ID   nullable.String `json:"id" validate:"max=1024"`
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataCloudMachine struct {
	Type nullable.String `json:"type" validate:"max=1024"`
}

type metadataCloudProject struct {
	ID   nullable.String `json:"id" validate:"max=1024"`
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataProcess struct {
	Argv  []string        `json:"argv"`
	Pid   nullable.Int    `json:"pid" validate:"required"`
	Ppid  nullable.Int    `json:"ppid"`
	Title nullable.String `json:"title" validate:"max=1024"`
}

type metadataService struct {
	Agent       metadataServiceAgent     `json:"agent" validate:"required"`
	Environment nullable.String          `json:"environment" validate:"max=1024"`
	Framework   metadataServiceFramework `json:"framework"`
	Language    metadataServiceLanguage  `json:"language"`
	Name        nullable.String          `json:"name" validate:"required,min=1,max=1024,pattern=regexpAlphaNumericExt"`
	Node        metadataServiceNode      `json:"node"`
	Runtime     metadataServiceRuntime   `json:"runtime"`
	Version     nullable.String          `json:"version" validate:"max=1024"`
}

type metadataServiceAgent struct {
	EphemeralID nullable.String `json:"ephemeral_id" validate:"max=1024"`
	Name        nullable.String `json:"name" validate:"required,min=1,max=1024"`
	Version     nullable.String `json:"version" validate:"required,max=1024"`
}

type metadataServiceFramework struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type metadataServiceLanguage struct {
	Name    nullable.String `json:"name" validate:"required,max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type metadataServiceNode struct {
	Name nullable.String `json:"configured_name" validate:"max=1024"`
}

type metadataServiceRuntime struct {
	Name    nullable.String `json:"name" validate:"required,max=1024"`
	Version nullable.String `json:"version" validate:"required,max=1024"`
}

type metadataSystem struct {
	Architecture       nullable.String          `json:"architecture" validate:"max=1024"`
	ConfiguredHostname nullable.String          `json:"configured_hostname" validate:"max=1024"`
	Container          metadataSystemContainer  `json:"container"`
	DetectedHostname   nullable.String          `json:"detected_hostname" validate:"max=1024"`
	HostnameDeprecated nullable.String          `json:"hostname" validate:"max=1024"`
	Kubernetes         metadataSystemKubernetes `json:"kubernetes"`
	Platform           nullable.String          `json:"platform" validate:"max=1024"`
}

type metadataSystemContainer struct {
	// `id` is the only field in `system.container`,
	// if `system.container:{}` is sent, it should be considered valid
	// if additional attributes are defined in the future, add the required tag
	ID nullable.String `json:"id"` //validate:"required"
}

type metadataSystemKubernetes struct {
	Namespace nullable.String              `json:"namespace" validate:"max=1024"`
	Node      metadataSystemKubernetesNode `json:"node"`
	Pod       metadataSystemKubernetesPod  `json:"pod"`
}

type metadataSystemKubernetesNode struct {
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataSystemKubernetesPod struct {
	Name nullable.String `json:"name" validate:"max=1024"`
	UID  nullable.String `json:"uid" validate:"max=1024"`
}

// NOTE(simitt):
// Event is only set by aggregator
// TimeseriesInstanceID is only set by aggregator
type metricset struct {
	Timestamp   nullable.TimeMicrosUnix         `json:"timestamp"`
	Samples     map[string]metricsetSampleValue `json:"samples" validate:"required,patternKeys=regexpNoAsteriskQuote"`
	Span        metricsetSpanRef                `json:"span"`
	Tags        common.MapStr                   `json:"tags" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
	Transaction metricsetTransactionRef         `json:"transaction"`
}

// TODO(axw/simitt): add support for ingesting counts/values (histogram metrics)
type metricsetSampleValue struct {
	Value nullable.Float64 `json:"value" validate:"required"`
}

// NOTE(simitt):
// span.DestinationService is only set by aggregator
type metricsetSpanRef struct {
	Subtype nullable.String `json:"subtype" validate:"max=1024"`
	Type    nullable.String `json:"type" validate:"max=1024"`
}

// NOTE(simitt):
// transaction.Result is only set by aggregator and rumV3
// transaction.Root is only set by aggregator
type metricsetTransactionRef struct {
	Name nullable.String `json:"name" validate:"max=1024"`
	Type nullable.String `json:"type" validate:"max=1024"`
}

type span struct {
	Action        nullable.String         `json:"action" validate:"max=1024"`
	ChildIDs      []string                `json:"child_ids" validate:"max=1024"`
	Context       spanContext             `json:"context"`
	Duration      nullable.Float64        `json:"duration" validate:"required,min=0"`
	ID            nullable.String         `json:"id" validate:"required,max=1024"`
	Name          nullable.String         `json:"name" validate:"required,max=1024"`
	Outcome       nullable.String         `json:"outcome" validate:"enum=enumOutcome"`
	ParentID      nullable.String         `json:"parent_id" validate:"required,max=1024"`
	SampleRate    nullable.Float64        `json:"sample_rate"`
	Stacktrace    []stacktraceFrame       `json:"stacktrace"`
	Start         nullable.Float64        `json:"start"`
	Subtype       nullable.String         `json:"subtype" validate:"max=1024"`
	Sync          nullable.Bool           `json:"sync"`
	Timestamp     nullable.TimeMicrosUnix `json:"timestamp"`
	TraceID       nullable.String         `json:"trace_id" validate:"required,max=1024"`
	TransactionID nullable.String         `json:"transaction_id" validate:"max=1024"`
	Type          nullable.String         `json:"type" validate:"required,max=1024"`
	_             struct{}                `validate:"requiredOneOf=start;timestamp"`
}

type spanContext struct {
	Database     spanContextDatabase    `json:"db"`
	Destination  spanContextDestination `json:"destination"`
	Experimental nullable.Interface     `json:"experimental"`
	HTTP         spanContextHTTP        `json:"http"`
	Message      contextMessage         `json:"message"`
	Service      contextService         `json:"service"`
	Tags         common.MapStr          `json:"tags" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
}

type spanContextDatabase struct {
	Instance     nullable.String `json:"instance"`
	Link         nullable.String `json:"link" validate:"max=1024"`
	RowsAffected nullable.Int    `json:"rows_affected"`
	Statement    nullable.String `json:"statement"`
	Type         nullable.String `json:"type"`
	User         nullable.String `json:"user"`
}

type spanContextDestination struct {
	Address nullable.String               `json:"address" validate:"max=1024"`
	Port    nullable.Int                  `json:"port"`
	Service spanContextDestinationService `json:"service"`
}

type spanContextDestinationService struct {
	Name     nullable.String `json:"name" validate:"required,max=1024"`
	Resource nullable.String `json:"resource" validate:"required,max=1024"`
	Type     nullable.String `json:"type" validate:"required,max=1024"`
}

type spanContextHTTP struct {
	Method     nullable.String         `json:"method" validate:"max=1024"`
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
	_            struct{}        `validate:"requiredOneOf=classname;filename"`
}

type transaction struct {
	Context        context                   `json:"context"`
	Duration       nullable.Float64          `json:"duration" validate:"required,min=0"`
	ID             nullable.String           `json:"id" validate:"required,max=1024"`
	Marks          transactionMarks          `json:"marks"`
	Name           nullable.String           `json:"name" validate:"max=1024"`
	Outcome        nullable.String           `json:"outcome" validate:"enum=enumOutcome"`
	ParentID       nullable.String           `json:"parent_id" validate:"max=1024"`
	Result         nullable.String           `json:"result" validate:"max=1024"`
	Sampled        nullable.Bool             `json:"sampled"`
	SampleRate     nullable.Float64          `json:"sample_rate"`
	SpanCount      transactionSpanCount      `json:"span_count" validate:"required"`
	Timestamp      nullable.TimeMicrosUnix   `json:"timestamp"`
	TraceID        nullable.String           `json:"trace_id" validate:"required,max=1024"`
	Type           nullable.String           `json:"type" validate:"required,max=1024"`
	UserExperience transactionUserExperience `json:"experience"`
}

type transactionMarks struct {
	Events map[string]transactionMarkEvents `json:"-" validate:"patternKeys=regexpNoDotAsteriskQuote"`
}

//TODO(simitt): generate
func (m *transactionMarks) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.Events)
}

type transactionMarkEvents struct {
	Measurements map[string]float64 `json:"-" validate:"patternKeys=regexpNoDotAsteriskQuote"`
}

//TODO(simitt): generate
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
	ID    nullable.Interface `json:"id" validate:"max=1024,inputTypes=string;int"`
	Email nullable.String    `json:"email" validate:"max=1024"`
	Name  nullable.String    `json:"username" validate:"max=1024"`
}
