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
	"regexp"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

var (
	regexpAlphaNumericExt    = regexp.MustCompile("^[a-zA-Z0-9 _-]+$")
	regexpNoDotAsteriskQuote = regexp.MustCompile("^[^.*\"]*$") //do not allow '.' '*' '"'

	enumOutcome = []string{"success", "failure", "unknown"}
)

// entry points

type errorRoot struct {
	Error errorEvent `json:"e" validate:"required"`
}

type metadataRoot struct {
	Metadata metadata `json:"m" validate:"required"`
}

type metricsetRoot struct {
	Metricset metricset `json:"me" validate:"required"`
}

type transactionRoot struct {
	Transaction transaction `json:"x" validate:"required"`
}

// other structs

type context struct {
	Custom   common.MapStr   `json:"cu" validate:"patternKeys=regexpNoDotAsteriskQuote"`
	Page     contextPage     `json:"p"`
	Request  contextRequest  `json:"q"`
	Response contextResponse `json:"r"`
	Service  contextService  `json:"se"`
	Tags     common.MapStr   `json:"g" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
	User     user            `json:"u"`
}

type contextPage struct {
	URL     nullable.String `json:"url"`
	Referer nullable.String `json:"rf"`
}

type contextRequest struct {
	Env         common.MapStr       `json:"en"`
	Headers     nullable.HTTPHeader `json:"he"`
	HTTPVersion nullable.String     `json:"hve" validate:"max=1024"`
	Method      nullable.String     `json:"mt" validate:"required,max=1024"`
}

type contextResponse struct {
	DecodedBodySize nullable.Float64    `json:"dbs"`
	EncodedBodySize nullable.Float64    `json:"ebs"`
	Headers         nullable.HTTPHeader `json:"he"`
	StatusCode      nullable.Int        `json:"sc"`
	TransferSize    nullable.Float64    `json:"ts"`
}

type contextService struct {
	Agent       contextServiceAgent     `json:"a"`
	Environment nullable.String         `json:"en" validate:"max=1024"`
	Framework   contextServiceFramework `json:"fw"`
	Language    contextServiceLanguage  `json:"la"`
	Name        nullable.String         `json:"n" validate:"max=1024,pattern=regexpAlphaNumericExt"`
	Runtime     contextServiceRuntime   `json:"ru"`
	Version     nullable.String         `json:"ve" validate:"max=1024"`
}

type contextServiceAgent struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type contextServiceFramework struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type contextServiceLanguage struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type contextServiceRuntime struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type errorEvent struct {
	Context       context                 `json:"c"`
	Culprit       nullable.String         `json:"cl" validate:"max=1024"`
	Exception     errorException          `json:"ex"`
	ID            nullable.String         `json:"id" validate:"required,max=1024"`
	Log           errorLog                `json:"log"`
	ParentID      nullable.String         `json:"pid" validate:"requiredIfAny=xid;tid,max=1024"`
	Timestamp     nullable.TimeMicrosUnix `json:"timestamp"`
	TraceID       nullable.String         `json:"tid" validate:"requiredIfAny=xid;pid,max=1024"`
	Transaction   errorTransactionRef     `json:"x"`
	TransactionID nullable.String         `json:"xid" validate:"max=1024"`
	_             struct{}                `validate:"requiredOneOf=ex;log"`
}

type errorException struct {
	Attributes common.MapStr      `json:"at"`
	Code       nullable.Interface `json:"cd" validate:"inputTypes=string;int,max=1024"`
	Cause      []errorException   `json:"ca"`
	Handled    nullable.Bool      `json:"hd"`
	Message    nullable.String    `json:"mg"`
	Module     nullable.String    `json:"mo" validate:"max=1024"`
	Stacktrace []stacktraceFrame  `json:"st"`
	Type       nullable.String    `json:"t" validate:"max=1024"`
	_          struct{}           `validate:"requiredOneOf=mg;t"`
}

type errorLog struct {
	Level        nullable.String   `json:"lv" validate:"max=1024"`
	LoggerName   nullable.String   `json:"ln" validate:"max=1024"`
	Message      nullable.String   `json:"mg" validate:"required"`
	ParamMessage nullable.String   `json:"pmg" validate:"max=1024"`
	Stacktrace   []stacktraceFrame `json:"st"`
}

type errorTransactionRef struct {
	Sampled nullable.Bool   `json:"sm"`
	Type    nullable.String `json:"t" validate:"max=1024"`
}

type metadata struct {
	Labels  common.MapStr   `json:"l" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
	Service metadataService `json:"se" validate:"required"`
	User    user            `json:"u"`
}

type metadataService struct {
	Agent       metadataServiceAgent     `json:"a" validate:"required"`
	Environment nullable.String          `json:"en" validate:"max=1024"`
	Framework   metadataServiceFramework `json:"fw"`
	Language    metadataServiceLanguage  `json:"la"`
	Name        nullable.String          `json:"n" validate:"required,min=1,max=1024,pattern=regexpAlphaNumericExt"`
	Runtime     metadataServiceRuntime   `json:"ru"`
	Version     nullable.String          `json:"ve" validate:"max=1024"`
}

type metadataServiceAgent struct {
	Name    nullable.String `json:"n" validate:"required,min=1,max=1024"`
	Version nullable.String `json:"ve" validate:"required,max=1024"`
}

type metadataServiceFramework struct {
	Name    nullable.String `json:"n" validate:"max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type metadataServiceLanguage struct {
	Name    nullable.String `json:"n" validate:"required,max=1024"`
	Version nullable.String `json:"ve" validate:"max=1024"`
}

type metadataServiceRuntime struct {
	Name    nullable.String `json:"n" validate:"required,max=1024"`
	Version nullable.String `json:"ve" validate:"required,max=1024"`
}

type metricset struct {
	Samples metricsetSamples `json:"sa" validate:"required"`
	Span    metricsetSpanRef `json:"y"`
	Tags    common.MapStr    `json:"g" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
}

type metricsetSamples struct {
	TransactionDurationCount  metricsetSampleValue `json:"xdc"`
	TransactionDurationSum    metricsetSampleValue `json:"xds"`
	TransactionBreakdownCount metricsetSampleValue `json:"xbc"`
	SpanSelfTimeCount         metricsetSampleValue `json:"ysc"`
	SpanSelfTimeSum           metricsetSampleValue `json:"yss"`
}

var (
	metricsetSamplesTransactionDurationCountName  = "transaction.duration.count"
	metricsetSamplesTransactionDurationSumName    = "transaction.duration.sum.us"
	metricsetSamplesTransactionBreakdownCountName = "transaction.breakdown.count"
	metricsetSamplesSpanSelfTimeCountName         = "span.self_time.count"
	metricsetSamplesSpanSelfTimeSumName           = "span.self_time.sum.us"
)

type metricsetSampleValue struct {
	Value nullable.Float64 `json:"v" validate:"required"`
}

type metricsetSpanRef struct {
	Subtype nullable.String `json:"su" validate:"max=1024"`
	Type    nullable.String `json:"t" validate:"max=1024"`
}

type span struct {
	Action      nullable.String   `json:"ac" validate:"max=1024"`
	Context     spanContext       `json:"c"`
	Duration    nullable.Float64  `json:"d" validate:"required,min=0"`
	ID          nullable.String   `json:"id" validate:"required,max=1024"`
	Name        nullable.String   `json:"n" validate:"required,max=1024"`
	Outcome     nullable.String   `json:"o" validate:"enum=enumOutcome"`
	ParentIndex nullable.Int      `json:"pi"`
	SampleRate  nullable.Float64  `json:"sr"`
	Stacktrace  []stacktraceFrame `json:"st"`
	Start       nullable.Float64  `json:"s" validate:"required"`
	Subtype     nullable.String   `json:"su" validate:"max=1024"`
	Sync        nullable.Bool     `json:"sy"`
	Type        nullable.String   `json:"t" validate:"required,max=1024"`
}

type spanContext struct {
	Destination spanContextDestination `json:"dt"`
	HTTP        spanContextHTTP        `json:"h"`
	Service     spanContextService     `json:"se"`
	Tags        common.MapStr          `json:"g" validate:"patternKeys=regexpNoDotAsteriskQuote,inputTypesVals=string;bool;number,maxVals=1024"`
}

type spanContextDestination struct {
	Address nullable.String               `json:"ad" validate:"max=1024"`
	Port    nullable.Int                  `json:"po"`
	Service spanContextDestinationService `json:"se"`
}

type spanContextDestinationService struct {
	Name     nullable.String `json:"n" validate:"required,max=1024"`
	Resource nullable.String `json:"rc" validate:"required,max=1024"`
	Type     nullable.String `json:"t" validate:"required,max=1024"`
}

type spanContextHTTP struct {
	Method     nullable.String         `json:"mt" validate:"max=1024"`
	StatusCode nullable.Int            `json:"sc"`
	URL        nullable.String         `json:"url"`
	Response   spanContextHTTPResponse `json:"r"`
}

type spanContextHTTPResponse struct {
	DecodedBodySize nullable.Float64 `json:"dbs"`
	EncodedBodySize nullable.Float64 `json:"ebs"`
	TransferSize    nullable.Float64 `json:"ts"`
}

type spanContextService struct {
	Agent contextServiceAgent `json:"a"`
	Name  nullable.String     `json:"n" validate:"max=1024,pattern=regexpAlphaNumericExt"`
}

type stacktraceFrame struct {
	AbsPath      nullable.String `json:"ap"`
	Classname    nullable.String `json:"cn"`
	ColumnNumber nullable.Int    `json:"co"`
	ContextLine  nullable.String `json:"cli"`
	Filename     nullable.String `json:"f" validate:"required"`
	Function     nullable.String `json:"fn"`
	LineNumber   nullable.Int    `json:"li"`
	Module       nullable.String `json:"mo"`
	PostContext  []string        `json:"poc"`
	PreContext   []string        `json:"prc"`
}

type transaction struct {
	Context        context                   `json:"c"`
	Duration       nullable.Float64          `json:"d" validate:"required,min=0"`
	ID             nullable.String           `json:"id" validate:"required,max=1024"`
	Marks          transactionMarks          `json:"k"`
	Name           nullable.String           `json:"n" validate:"max=1024"`
	Outcome        nullable.String           `json:"o" validate:"enum=enumOutcome"`
	ParentID       nullable.String           `json:"pid" validate:"max=1024"`
	Result         nullable.String           `json:"rt" validate:"max=1024"`
	Sampled        nullable.Bool             `json:"sm"`
	SampleRate     nullable.Float64          `json:"sr"`
	SpanCount      transactionSpanCount      `json:"yc" validate:"required"`
	TraceID        nullable.String           `json:"tid" validate:"required,max=1024"`
	Type           nullable.String           `json:"t" validate:"required,max=1024"`
	UserExperience transactionUserExperience `json:"exp"`
	Metricsets     []metricset               `json:"me"`
	Spans          []span                    `json:"y"`
}

type transactionMarks struct {
	Events map[string]transactionMarkEvents `json:"-" validate:"patternKeys=regexpNoDotAsteriskQuote"`
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
	Measurements map[string]float64 `json:"-" validate:"patternKeys=regexpNoDotAsteriskQuote"`
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
	Dropped nullable.Int `json:"dd"`
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
	Count nullable.Int     `json:"count" validate:"required,min=0"`
	Sum   nullable.Float64 `json:"sum" validate:"required,min=0"`
	Max   nullable.Float64 `json:"max" validate:"required,min=0"`
}

type user struct {
	ID    nullable.Interface `json:"id" validate:"max=1024,inputTypes=string;int"`
	Email nullable.String    `json:"em" validate:"max=1024"`
	Name  nullable.String    `json:"un" validate:"max=1024"`
}
