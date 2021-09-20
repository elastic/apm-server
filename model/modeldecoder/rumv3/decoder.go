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
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecoderutil"
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
)

var (
	errorRootPool = sync.Pool{
		New: func() interface{} {
			return &errorRoot{}
		},
	}
	metadataRootPool = sync.Pool{
		New: func() interface{} {
			return &metadataRoot{}
		},
	}
	transactionRootPool = sync.Pool{
		New: func() interface{} {
			return &transactionRoot{}
		},
	}
)

func fetchErrorRoot() *errorRoot {
	return errorRootPool.Get().(*errorRoot)
}

func releaseErrorRoot(root *errorRoot) {
	root.Reset()
	errorRootPool.Put(root)
}

func fetchMetadataRoot() *metadataRoot {
	return metadataRootPool.Get().(*metadataRoot)
}

func releaseMetadataRoot(m *metadataRoot) {
	m.Reset()
	metadataRootPool.Put(m)
}

func fetchTransactionRoot() *transactionRoot {
	return transactionRootPool.Get().(*transactionRoot)
}

func releaseTransactionRoot(m *transactionRoot) {
	m.Reset()
	transactionRootPool.Put(m)
}

// DecodeNestedMetadata decodes metadata from d, updating out.
func DecodeNestedMetadata(d decoder.Decoder, out *model.APMEvent) error {
	root := fetchMetadataRoot()
	defer releaseMetadataRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToMetadataModel(&root.Metadata, out)
	return nil
}

// DecodeNestedError decodes an error from d, appending it to batch.
//
// DecodeNestedError should be used when the stream in the decoder contains the `error` key
func DecodeNestedError(d decoder.Decoder, input *modeldecoder.Input, batch *model.Batch) error {
	root := fetchErrorRoot()
	defer releaseErrorRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base
	mapToErrorModel(&root.Error, &event)
	*batch = append(*batch, event)
	return nil
}

// DecodeNestedTransaction a transaction and zero or more nested spans and
// metricsets, appending them to batch.
//
// DecodeNestedTransaction should be used when the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, batch *model.Batch) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}

	transaction := input.Base
	mapToTransactionModel(&root.Transaction, &transaction)
	*batch = append(*batch, transaction)

	for _, m := range root.Transaction.Metricsets {
		event := input.Base
		event.Transaction = &model.Transaction{
			Name: transaction.Transaction.Name,
			Type: transaction.Transaction.Type,
		}
		mapToTransactionMetricsetModel(&m, &event)
		*batch = append(*batch, event)
	}

	offset := len(*batch)
	for _, s := range root.Transaction.Spans {
		event := input.Base
		mapToSpanModel(&s, &event)
		event.Transaction = &model.Transaction{ID: transaction.Transaction.ID}
		event.Parent.ID = transaction.Transaction.ID // may be overridden later
		event.Trace = transaction.Trace
		*batch = append(*batch, event)
	}
	spans := (*batch)[offset:]
	for i, s := range root.Transaction.Spans {
		if s.ParentIndex.IsSet() && s.ParentIndex.Val >= 0 && s.ParentIndex.Val < len(spans) {
			spans[i].Parent.ID = spans[s.ParentIndex.Val].Span.ID
		}
	}
	return nil
}

func mapToErrorModel(from *errorEvent, event *model.APMEvent) {
	out := &model.Error{}
	event.Error = out
	event.Processor = model.ErrorProcessor

	// overwrite metadata with event specific information
	mapToServiceModel(from.Context.Service, &event.Service)
	mapToAgentModel(from.Context.Service.Agent, &event.Agent)
	overwriteUserInMetadataModel(from.Context.User, event)
	mapToUserAgentModel(from.Context.Request.Headers, &event.UserAgent)

	// map errorEvent specific data
	if from.Context.IsSet() {
		if len(from.Context.Tags) > 0 {
			event.Labels = modeldecoderutil.MergeLabels(
				event.Labels,
				modeldecoderutil.NormalizeLabelValues(from.Context.Tags),
			)
		}
		if from.Context.Request.IsSet() {
			event.HTTP.Request = &model.HTTPRequest{}
			mapToRequestModel(from.Context.Request, event.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				event.HTTP.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Response.IsSet() {
			event.HTTP.Response = &model.HTTPResponse{}
			mapToResponseModel(from.Context.Response, event.HTTP.Response)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() {
				event.URL = model.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.HTTP.Request == nil {
					event.HTTP.Request = &model.HTTPRequest{}
				}
				event.HTTP.Request.Referrer = from.Context.Page.Referer.Val
			}
		}
		if len(from.Context.Custom) > 0 {
			out.Custom = modeldecoderutil.NormalizeLabelValues(from.Context.Custom.Clone())
		}
	}
	if from.Culprit.IsSet() {
		out.Culprit = from.Culprit.Val
	}
	if from.Exception.IsSet() {
		out.Exception = &model.Exception{}
		mapToExceptionModel(from.Exception, out.Exception)
	}
	if from.ID.IsSet() {
		out.ID = from.ID.Val
	}
	if from.Log.IsSet() {
		log := model.Log{}
		if from.Log.Level.IsSet() {
			log.Level = from.Log.Level.Val
		}
		loggerName := "default"
		if from.Log.LoggerName.IsSet() {
			loggerName = from.Log.LoggerName.Val

		}
		log.LoggerName = loggerName
		if from.Log.Message.IsSet() {
			log.Message = from.Log.Message.Val
		}
		if from.Log.ParamMessage.IsSet() {
			log.ParamMessage = from.Log.ParamMessage.Val
		}
		if len(from.Log.Stacktrace) > 0 {
			log.Stacktrace = make(model.Stacktrace, len(from.Log.Stacktrace))
			mapToStracktraceModel(from.Log.Stacktrace, log.Stacktrace)
		}
		out.Log = &log
	}
	if from.ParentID.IsSet() {
		event.Parent.ID = from.ParentID.Val
	}
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = from.Timestamp.Val
	}
	if from.TraceID.IsSet() {
		event.Trace.ID = from.TraceID.Val
	}
	if from.Transaction.IsSet() {
		event.Transaction = &model.Transaction{}
		if from.Transaction.Sampled.IsSet() {
			event.Transaction.Sampled = from.Transaction.Sampled.Val
		}
		if from.Transaction.Type.IsSet() {
			event.Transaction.Type = from.Transaction.Type.Val
		}
		if from.TransactionID.IsSet() {
			event.Transaction.ID = from.TransactionID.Val
		}
	}
}

func mapToExceptionModel(from errorException, out *model.Exception) {
	if !from.IsSet() {
		return
	}
	if len(from.Attributes) > 0 {
		out.Attributes = from.Attributes.Clone()
	}
	if from.Code.IsSet() {
		out.Code = modeldecoderutil.ExceptionCodeString(from.Code.Val)
	}
	if len(from.Cause) > 0 {
		out.Cause = make([]model.Exception, len(from.Cause))
		for i := 0; i < len(from.Cause); i++ {
			var ex model.Exception
			mapToExceptionModel(from.Cause[i], &ex)
			out.Cause[i] = ex
		}
	}
	if from.Handled.IsSet() {
		out.Handled = &from.Handled.Val
	}
	if from.Message.IsSet() {
		out.Message = from.Message.Val
	}
	if from.Module.IsSet() {
		out.Module = from.Module.Val
	}
	if len(from.Stacktrace) > 0 {
		out.Stacktrace = make(model.Stacktrace, len(from.Stacktrace))
		mapToStracktraceModel(from.Stacktrace, out.Stacktrace)
	}
	if from.Type.IsSet() {
		out.Type = from.Type.Val
	}
}

func mapToMetadataModel(m *metadata, out *model.APMEvent) {
	// Labels
	if len(m.Labels) > 0 {
		out.Labels = modeldecoderutil.NormalizeLabelValues(m.Labels.Clone())
	}

	// Service
	if m.Service.Agent.Name.IsSet() {
		out.Agent.Name = m.Service.Agent.Name.Val
	}
	if m.Service.Agent.Version.IsSet() {
		out.Agent.Version = m.Service.Agent.Version.Val
	}
	if m.Service.Environment.IsSet() {
		out.Service.Environment = m.Service.Environment.Val
	}
	if m.Service.Framework.Name.IsSet() {
		out.Service.Framework.Name = m.Service.Framework.Name.Val
	}
	if m.Service.Framework.Version.IsSet() {
		out.Service.Framework.Version = m.Service.Framework.Version.Val
	}
	if m.Service.Language.Name.IsSet() {
		out.Service.Language.Name = m.Service.Language.Name.Val
	}
	if m.Service.Language.Version.IsSet() {
		out.Service.Language.Version = m.Service.Language.Version.Val
	}
	if m.Service.Name.IsSet() {
		out.Service.Name = m.Service.Name.Val
	}
	if m.Service.Runtime.Name.IsSet() {
		out.Service.Runtime.Name = m.Service.Runtime.Name.Val
	}
	if m.Service.Runtime.Version.IsSet() {
		out.Service.Runtime.Version = m.Service.Runtime.Version.Val
	}
	if m.Service.Version.IsSet() {
		out.Service.Version = m.Service.Version.Val
	}

	// User
	if m.User.Domain.IsSet() {
		out.User.Domain = fmt.Sprint(m.User.Domain.Val)
	}
	if m.User.ID.IsSet() {
		out.User.ID = fmt.Sprint(m.User.ID.Val)
	}
	if m.User.Email.IsSet() {
		out.User.Email = m.User.Email.Val
	}
	if m.User.Name.IsSet() {
		out.User.Name = m.User.Name.Val
	}

	// Network
	if m.Network.Connection.Type.IsSet() {
		out.Network.Connection.Type = m.Network.Connection.Type.Val
	}
}

func mapToTransactionMetricsetModel(from *transactionMetricset, event *model.APMEvent) {
	event.Metricset = &model.Metricset{}
	event.Processor = model.MetricsetProcessor

	if from.Span.IsSet() {
		event.Span = &model.Span{}
		if from.Span.Subtype.IsSet() {
			event.Span.Subtype = from.Span.Subtype.Val
		}
		if from.Span.Type.IsSet() {
			event.Span.Type = from.Span.Type.Val
		}
	}

	if from.Samples.IsSet() {
		if event.Transaction != nil {
			if value := from.Samples.TransactionBreakdownCount.Value; value.IsSet() {
				event.Transaction.BreakdownCount = int(value.Val)
			}
		}
		if event.Span != nil {
			if value := from.Samples.SpanSelfTimeCount.Value; value.IsSet() {
				event.Span.SelfTime.Count = int(value.Val)
			}
			if value := from.Samples.SpanSelfTimeSum.Value; value.IsSet() {
				event.Span.SelfTime.Sum = time.Duration(value.Val * 1000)
			}
		}
	}
}

func mapToResponseModel(from contextResponse, out *model.HTTPResponse) {
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToMap(from.Headers.Val.Clone())
	}
	if from.StatusCode.IsSet() {
		out.StatusCode = from.StatusCode.Val
	}
	if from.TransferSize.IsSet() {
		val := from.TransferSize.Val
		out.TransferSize = &val
	}
	if from.EncodedBodySize.IsSet() {
		val := from.EncodedBodySize.Val
		out.EncodedBodySize = &val
	}
	if from.DecodedBodySize.IsSet() {
		val := from.DecodedBodySize.Val
		out.DecodedBodySize = &val
	}
}

func mapToRequestModel(from contextRequest, out *model.HTTPRequest) {
	if from.Method.IsSet() {
		out.Method = from.Method.Val
	}
	if len(from.Env) > 0 {
		out.Env = from.Env.Clone()
	}
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToMap(from.Headers.Val.Clone())
	}
}

func mapToServiceModel(from contextService, out *model.Service) {
	if from.Environment.IsSet() {
		out.Environment = from.Environment.Val
	}
	if from.Framework.Name.IsSet() {
		out.Framework.Name = from.Framework.Name.Val
	}
	if from.Framework.Version.IsSet() {
		out.Framework.Version = from.Framework.Version.Val
	}
	if from.Language.Name.IsSet() {
		out.Language.Name = from.Language.Name.Val
	}
	if from.Language.Version.IsSet() {
		out.Language.Version = from.Language.Version.Val
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Runtime.Name.IsSet() {
		out.Runtime.Name = from.Runtime.Name.Val
	}
	if from.Runtime.Version.IsSet() {
		out.Runtime.Version = from.Runtime.Version.Val
	}
	if from.Version.IsSet() {
		out.Version = from.Version.Val
	}
}

func mapToAgentModel(from contextServiceAgent, out *model.Agent) {
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Version.IsSet() {
		out.Version = from.Version.Val
	}
}

func mapToSpanModel(from *span, event *model.APMEvent) {
	out := &model.Span{}
	event.Span = out
	event.Processor = model.SpanProcessor

	// map span specific data
	if !from.Action.IsSet() && !from.Subtype.IsSet() {
		sep := "."
		typ := strings.Split(from.Type.Val, sep)
		out.Type = typ[0]
		if len(typ) > 1 {
			out.Subtype = typ[1]
			if len(typ) > 2 {
				out.Action = strings.Join(typ[2:], sep)
			}
		}
	} else {
		if from.Action.IsSet() {
			out.Action = from.Action.Val
		}
		if from.Subtype.IsSet() {
			out.Subtype = from.Subtype.Val
		}
		if from.Type.IsSet() {
			out.Type = from.Type.Val
		}
	}
	if from.Context.Destination.Address.IsSet() || from.Context.Destination.Port.IsSet() {
		if from.Context.Destination.Address.IsSet() {
			event.Destination.Address = from.Context.Destination.Address.Val
		}
		if from.Context.Destination.Port.IsSet() {
			event.Destination.Port = from.Context.Destination.Port.Val
		}
	}
	if from.Context.Destination.Service.IsSet() {
		service := model.DestinationService{}
		if from.Context.Destination.Service.Name.IsSet() {
			service.Name = from.Context.Destination.Service.Name.Val
		}
		if from.Context.Destination.Service.Resource.IsSet() {
			service.Resource = from.Context.Destination.Service.Resource.Val
		}
		if from.Context.Destination.Service.Type.IsSet() {
			service.Type = from.Context.Destination.Service.Type.Val
		}
		out.DestinationService = &service
	}
	if from.Context.HTTP.IsSet() {
		var response model.HTTPResponse
		if from.Context.HTTP.Method.IsSet() {
			event.HTTP.Request = &model.HTTPRequest{}
			event.HTTP.Request.Method = from.Context.HTTP.Method.Val
		}
		if from.Context.HTTP.StatusCode.IsSet() {
			event.HTTP.Response = &response
			event.HTTP.Response.StatusCode = from.Context.HTTP.StatusCode.Val
		}
		if from.Context.HTTP.URL.IsSet() {
			event.URL.Original = from.Context.HTTP.URL.Val
		}
		if from.Context.HTTP.Response.IsSet() {
			event.HTTP.Response = &response
			if from.Context.HTTP.Response.DecodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.DecodedBodySize.Val
				event.HTTP.Response.DecodedBodySize = &val
			}
			if from.Context.HTTP.Response.EncodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.EncodedBodySize.Val
				event.HTTP.Response.EncodedBodySize = &val
			}
			if from.Context.HTTP.Response.TransferSize.IsSet() {
				val := from.Context.HTTP.Response.TransferSize.Val
				event.HTTP.Response.TransferSize = &val
			}
		}
	}
	if from.Context.Service.IsSet() {
		if from.Context.Service.Name.IsSet() {
			event.Service.Name = from.Context.Service.Name.Val
		}
	}
	if len(from.Context.Tags) > 0 {
		event.Labels = modeldecoderutil.MergeLabels(
			event.Labels,
			modeldecoderutil.NormalizeLabelValues(from.Context.Tags),
		)
	}
	if from.Duration.IsSet() {
		duration := time.Duration(from.Duration.Val * float64(time.Millisecond))
		event.Event.Duration = duration
	}
	if from.ID.IsSet() {
		out.ID = from.ID.Val
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Outcome.IsSet() {
		event.Event.Outcome = from.Outcome.Val
	} else {
		if from.Context.HTTP.StatusCode.IsSet() {
			statusCode := from.Context.HTTP.StatusCode.Val
			if statusCode >= http.StatusBadRequest {
				event.Event.Outcome = "failure"
			} else {
				event.Event.Outcome = "success"
			}
		} else {
			event.Event.Outcome = "unknown"
		}
	}
	if from.SampleRate.IsSet() && from.SampleRate.Val > 0 {
		out.RepresentativeCount = 1 / from.SampleRate.Val
	}
	if len(from.Stacktrace) > 0 {
		out.Stacktrace = make(model.Stacktrace, len(from.Stacktrace))
		mapToStracktraceModel(from.Stacktrace, out.Stacktrace)
	}
	if from.Start.IsSet() {
		val := from.Start.Val
		out.Start = &val
	}
	if from.Sync.IsSet() {
		val := from.Sync.Val
		out.Sync = &val
	}
	if from.Start.IsSet() {
		// event.Timestamp is initialized to the time the payload was
		// received by apm-server; offset that by "start" milliseconds
		// for RUM.
		event.Timestamp = event.Timestamp.Add(
			time.Duration(float64(time.Millisecond) * from.Start.Val),
		)
	}
}

func mapToStracktraceModel(from []stacktraceFrame, out model.Stacktrace) {
	for idx, eventFrame := range from {
		fr := model.StacktraceFrame{}
		if eventFrame.AbsPath.IsSet() {
			fr.AbsPath = eventFrame.AbsPath.Val
		}
		if eventFrame.Classname.IsSet() {
			fr.Classname = eventFrame.Classname.Val
		}
		if eventFrame.ColumnNumber.IsSet() {
			val := eventFrame.ColumnNumber.Val
			fr.Colno = &val
		}
		if eventFrame.ContextLine.IsSet() {
			fr.ContextLine = eventFrame.ContextLine.Val
		}
		if eventFrame.Filename.IsSet() {
			fr.Filename = eventFrame.Filename.Val
		}
		if eventFrame.Function.IsSet() {
			fr.Function = eventFrame.Function.Val
		}
		if eventFrame.LineNumber.IsSet() {
			val := eventFrame.LineNumber.Val
			fr.Lineno = &val
		}
		if eventFrame.Module.IsSet() {
			fr.Module = eventFrame.Module.Val
		}
		if len(eventFrame.PostContext) > 0 {
			fr.PostContext = make([]string, len(eventFrame.PostContext))
			copy(fr.PostContext, eventFrame.PostContext)
		}
		if len(eventFrame.PreContext) > 0 {
			fr.PreContext = make([]string, len(eventFrame.PreContext))
			copy(fr.PreContext, eventFrame.PreContext)
		}
		out[idx] = &fr
	}
}

func mapToTransactionModel(from *transaction, event *model.APMEvent) {
	out := &model.Transaction{}
	event.Transaction = out
	event.Processor = model.TransactionProcessor

	// overwrite metadata with event specific information
	mapToServiceModel(from.Context.Service, &event.Service)
	mapToAgentModel(from.Context.Service.Agent, &event.Agent)
	overwriteUserInMetadataModel(from.Context.User, event)
	mapToUserAgentModel(from.Context.Request.Headers, &event.UserAgent)

	// map transaction specific data
	if from.Context.IsSet() {
		if len(from.Context.Custom) > 0 {
			out.Custom = modeldecoderutil.NormalizeLabelValues(from.Context.Custom.Clone())
		}
		if len(from.Context.Tags) > 0 {
			event.Labels = modeldecoderutil.MergeLabels(
				event.Labels,
				modeldecoderutil.NormalizeLabelValues(from.Context.Tags),
			)
		}
		if from.Context.Request.IsSet() {
			event.HTTP.Request = &model.HTTPRequest{}
			mapToRequestModel(from.Context.Request, event.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				event.HTTP.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Response.IsSet() {
			event.HTTP.Response = &model.HTTPResponse{}
			mapToResponseModel(from.Context.Response, event.HTTP.Response)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() {
				event.URL = model.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.HTTP.Request == nil {
					event.HTTP.Request = &model.HTTPRequest{}
				}
				event.HTTP.Request.Referrer = from.Context.Page.Referer.Val
			}
		}
	}
	if from.Duration.IsSet() {
		duration := time.Duration(from.Duration.Val * float64(time.Millisecond))
		event.Event.Duration = duration
	}
	if from.ID.IsSet() {
		out.ID = from.ID.Val
	}
	if from.Marks.IsSet() {
		out.Marks = make(model.TransactionMarks, len(from.Marks.Events))
		for event, val := range from.Marks.Events {
			if len(val.Measurements) > 0 {
				out.Marks[event] = model.TransactionMark(val.Measurements)
			}
		}
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Outcome.IsSet() {
		event.Event.Outcome = from.Outcome.Val
	} else {
		if from.Context.Response.StatusCode.IsSet() {
			statusCode := from.Context.Response.StatusCode.Val
			if statusCode >= http.StatusInternalServerError {
				event.Event.Outcome = "failure"
			} else {
				event.Event.Outcome = "success"
			}
		} else {
			event.Event.Outcome = "unknown"
		}
	}
	if from.ParentID.IsSet() {
		event.Parent.ID = from.ParentID.Val
	}
	if from.Result.IsSet() {
		out.Result = from.Result.Val
	}

	sampled := true
	if from.Sampled.IsSet() {
		sampled = from.Sampled.Val
	}
	out.Sampled = sampled
	if from.SampleRate.IsSet() {
		if from.SampleRate.Val > 0 {
			out.RepresentativeCount = 1 / from.SampleRate.Val
		}
	} else {
		out.RepresentativeCount = 1
	}
	if from.Session.ID.IsSet() {
		event.Session.ID = from.Session.ID.Val
		event.Session.Sequence = from.Session.Sequence.Val
	}
	if from.SpanCount.Dropped.IsSet() {
		dropped := from.SpanCount.Dropped.Val
		out.SpanCount.Dropped = &dropped
	}
	if from.SpanCount.Started.IsSet() {
		started := from.SpanCount.Started.Val
		out.SpanCount.Started = &started
	}
	if from.TraceID.IsSet() {
		event.Trace.ID = from.TraceID.Val
	}
	if from.Type.IsSet() {
		out.Type = from.Type.Val
	}
	if from.UserExperience.IsSet() {
		out.UserExperience = &model.UserExperience{
			CumulativeLayoutShift: -1,
			FirstInputDelay:       -1,
			TotalBlockingTime:     -1,
			Longtask:              model.LongtaskMetrics{Count: -1},
		}
		if from.UserExperience.CumulativeLayoutShift.IsSet() {
			out.UserExperience.CumulativeLayoutShift = from.UserExperience.CumulativeLayoutShift.Val
		}
		if from.UserExperience.FirstInputDelay.IsSet() {
			out.UserExperience.FirstInputDelay = from.UserExperience.FirstInputDelay.Val
		}
		if from.UserExperience.TotalBlockingTime.IsSet() {
			out.UserExperience.TotalBlockingTime = from.UserExperience.TotalBlockingTime.Val
		}
		if from.UserExperience.Longtask.IsSet() {
			out.UserExperience.Longtask = model.LongtaskMetrics{
				Count: from.UserExperience.Longtask.Count.Val,
				Sum:   from.UserExperience.Longtask.Sum.Val,
				Max:   from.UserExperience.Longtask.Max.Val,
			}
		}
	}
}

func mapToUserAgentModel(from nullable.HTTPHeader, out *model.UserAgent) {
	// overwrite userAgent information if available
	if from.IsSet() {
		if h := from.Val.Values(textproto.CanonicalMIMEHeaderKey("User-Agent")); len(h) > 0 {
			out.Original = strings.Join(h, ", ")
		}
	}
}

func overwriteUserInMetadataModel(from user, out *model.APMEvent) {
	// overwrite User specific values if set
	// either populate all User fields or none to avoid mixing
	// different user data
	if !from.Domain.IsSet() && !from.ID.IsSet() && !from.Email.IsSet() && !from.Name.IsSet() {
		return
	}
	out.User = model.User{}
	if from.Domain.IsSet() {
		out.User.Domain = fmt.Sprint(from.Domain.Val)
	}
	if from.ID.IsSet() {
		out.User.ID = fmt.Sprint(from.ID.Val)
	}
	if from.Email.IsSet() {
		out.User.Email = from.Email.Val
	}
	if from.Name.IsSet() {
		out.User.Name = from.Name.Val
	}
}
