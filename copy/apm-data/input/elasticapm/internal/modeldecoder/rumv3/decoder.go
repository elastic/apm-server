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

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecoderutil"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/nullable"
	"github.com/elastic/apm-data/model/modelpb"
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
func DecodeNestedMetadata(d decoder.Decoder, out *modelpb.APMEvent) error {
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
func DecodeNestedError(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := fetchErrorRoot()
	defer releaseErrorRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base.CloneVT()
	mapToErrorModel(&root.Error, event)
	*batch = append(*batch, event)
	return nil
}

// DecodeNestedTransaction a transaction and zero or more nested spans and
// metricsets, appending them to batch.
//
// DecodeNestedTransaction should be used when the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	transaction := input.Base.CloneVT()
	mapToTransactionModel(&root.Transaction, transaction)
	*batch = append(*batch, transaction)

	for _, m := range root.Transaction.Metricsets {
		event := input.Base.CloneVT()
		event.Transaction = &modelpb.Transaction{}
		event.Transaction.Name = transaction.Transaction.Name
		event.Transaction.Type = transaction.Transaction.Type
		if mapToTransactionMetricsetModel(&m, event) {
			*batch = append(*batch, event)
		}
	}

	offset := len(*batch)
	for _, s := range root.Transaction.Spans {
		event := input.Base.CloneVT()
		mapToSpanModel(&s, event)
		event.Transaction = &modelpb.Transaction{}
		event.Transaction.Id = transaction.Transaction.Id
		event.ParentId = transaction.GetTransaction().GetId() // may be overridden later
		event.Trace = transaction.Trace.CloneVT()
		*batch = append(*batch, event)
	}
	spans := (*batch)[offset:]
	for i, s := range root.Transaction.Spans {
		if s.ParentIndex.IsSet() && s.ParentIndex.Val >= 0 && s.ParentIndex.Val < len(spans) {
			if e := spans[s.ParentIndex.Val]; e != nil {
				spans[i].ParentId = e.Span.Id
			}
		}
	}
	return nil
}

func mapToErrorModel(from *errorEvent, event *modelpb.APMEvent) {
	out := modelpb.Error{}
	event.Error = &out

	// overwrite metadata with event specific information
	if from.Context.Service.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		mapToServiceModel(from.Context.Service, event.Service)
	}
	if from.Context.Service.Agent.IsSet() {
		if event.Agent == nil {
			event.Agent = &modelpb.Agent{}
		}
		mapToAgentModel(from.Context.Service.Agent, event.Agent)
	}
	overwriteUserInMetadataModel(from.Context.User, event)
	if from.Context.Request.Headers.IsSet() {
		if event.UserAgent == nil {
			event.UserAgent = &modelpb.UserAgent{}
		}
		mapToUserAgentModel(from.Context.Request.Headers, event.UserAgent)
	}

	// map errorEvent specific data
	if from.Context.IsSet() {
		if len(from.Context.Tags) > 0 {
			modeldecoderutil.MergeLabels(from.Context.Tags, event)
		}
		if from.Context.Request.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			event.Http.Request = &modelpb.HTTPRequest{}
			mapToRequestModel(from.Context.Request, event.Http.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				event.Http.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Response.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			event.Http.Response = &modelpb.HTTPResponse{}
			mapToResponseModel(from.Context.Response, event.Http.Response)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() {
				event.Url = modelpb.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.Http == nil {
					event.Http = &modelpb.HTTP{}
				}
				if event.Http.Request == nil {
					event.Http.Request = &modelpb.HTTPRequest{}
				}
				event.Http.Request.Referrer = from.Context.Page.Referer.Val
			}
		}
		if len(from.Context.Custom) > 0 {
			out.Custom = modeldecoderutil.ToKv(from.Context.Custom, out.Custom)
		}
	}
	if from.Culprit.IsSet() {
		out.Culprit = from.Culprit.Val
	}
	if from.Exception.IsSet() {
		out.Exception = &modelpb.Exception{}
		mapToExceptionModel(&from.Exception, out.Exception)
	}
	if from.ID.IsSet() {
		out.Id = from.ID.Val
	}
	if from.Log.IsSet() {
		log := modelpb.ErrorLog{}
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
			log.Stacktrace = modeldecoderutil.ResliceAndPopulateNil(
				log.Stacktrace,
				len(from.Log.Stacktrace),
				modelpb.StacktraceFrameFromVTPool,
			)
			mapToStracktraceModel(from.Log.Stacktrace, log.Stacktrace)
		}
		out.Log = &log
	}
	if from.ParentID.IsSet() {
		event.ParentId = from.ParentID.Val
	}
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = modelpb.FromTime(from.Timestamp.Val)
	}
	if from.TraceID.IsSet() {
		event.Trace = &modelpb.Trace{}
		event.Trace.Id = from.TraceID.Val
	}
	if from.Transaction.IsSet() {
		event.Transaction = &modelpb.Transaction{}
		if from.Transaction.Sampled.IsSet() {
			event.Transaction.Sampled = from.Transaction.Sampled.Val
		}
		if from.Transaction.Name.IsSet() {
			event.Transaction.Name = from.Transaction.Name.Val
		}
		if from.Transaction.Type.IsSet() {
			event.Transaction.Type = from.Transaction.Type.Val
		}
		if from.TransactionID.IsSet() {
			event.Transaction.Id = from.TransactionID.Val
		}
	}
}

func mapToExceptionModel(from *errorException, out *modelpb.Exception) {
	if len(from.Attributes) > 0 {
		out.Attributes = modeldecoderutil.ToKv(from.Attributes, out.Attributes)
	}
	if from.Code.IsSet() {
		out.Code = modeldecoderutil.ExceptionCodeString(from.Code.Val)
	}
	if len(from.Cause) > 0 {
		out.Cause = modeldecoderutil.ResliceAndPopulateNil(
			out.Cause,
			len(from.Cause),
			modelpb.ExceptionFromVTPool,
		)
		for i := 0; i < len(from.Cause); i++ {
			mapToExceptionModel(&from.Cause[i], out.Cause[i])
		}
	}
	if from.Handled.IsSet() {
		handled := from.Handled.Val
		out.Handled = &handled
	}
	if from.Message.IsSet() {
		out.Message = from.Message.Val
	}
	if from.Module.IsSet() {
		out.Module = from.Module.Val
	}
	if len(from.Stacktrace) > 0 {
		out.Stacktrace = modeldecoderutil.ResliceAndPopulateNil(
			out.Stacktrace,
			len(from.Stacktrace),
			modelpb.StacktraceFrameFromVTPool,
		)
		mapToStracktraceModel(from.Stacktrace, out.Stacktrace)
	}
	if from.Type.IsSet() {
		out.Type = from.Type.Val
	}
}

func mapToMetadataModel(m *metadata, out *modelpb.APMEvent) {
	// Labels
	if len(m.Labels) > 0 {
		modeldecoderutil.GlobalLabelsFrom(m.Labels, out)
	}

	// Service
	if m.Service.Agent.IsSet() {
		if out.Agent == nil {
			out.Agent = &modelpb.Agent{}
		}
		if m.Service.Agent.Name.IsSet() {
			out.Agent.Name = m.Service.Agent.Name.Val
		}
		if m.Service.Agent.Version.IsSet() {
			out.Agent.Version = m.Service.Agent.Version.Val
		}
	}
	if m.Service.IsSet() {
		if out.Service == nil {
			out.Service = &modelpb.Service{}
		}
		if m.Service.Environment.IsSet() {
			out.Service.Environment = m.Service.Environment.Val
		}
		if m.Service.Framework.IsSet() {
			if out.Service.Framework == nil {
				out.Service.Framework = &modelpb.Framework{}
			}
			if m.Service.Framework.Name.IsSet() {
				out.Service.Framework.Name = m.Service.Framework.Name.Val
			}
			if m.Service.Framework.Version.IsSet() {
				out.Service.Framework.Version = m.Service.Framework.Version.Val
			}
		}
		if m.Service.Language.IsSet() {
			if out.Service.Language == nil {
				out.Service.Language = &modelpb.Language{}
			}
			if m.Service.Language.Name.IsSet() {
				out.Service.Language.Name = m.Service.Language.Name.Val
			}
			if m.Service.Language.Version.IsSet() {
				out.Service.Language.Version = m.Service.Language.Version.Val
			}
		}
		if m.Service.Name.IsSet() {
			out.Service.Name = m.Service.Name.Val
		}
		if m.Service.Runtime.IsSet() {
			if out.Service.Runtime == nil {
				out.Service.Runtime = &modelpb.Runtime{}
			}
			if m.Service.Runtime.Name.IsSet() {
				out.Service.Runtime.Name = m.Service.Runtime.Name.Val
			}
			if m.Service.Runtime.Version.IsSet() {
				out.Service.Runtime.Version = m.Service.Runtime.Version.Val
			}
		}
		if m.Service.Version.IsSet() {
			out.Service.Version = m.Service.Version.Val
		}
	}

	// User
	if m.User.IsSet() {
		if out.User == nil {
			out.User = &modelpb.User{}
		}
		if m.User.Domain.IsSet() {
			out.User.Domain = m.User.Domain.Val
		}
		if m.User.ID.IsSet() {
			out.User.Id = fmt.Sprint(m.User.ID.Val)
		}
		if m.User.Email.IsSet() {
			out.User.Email = m.User.Email.Val
		}
		if m.User.Name.IsSet() {
			out.User.Name = m.User.Name.Val
		}
	}

	// Network
	if m.Network.Connection.Type.IsSet() {
		if out.Network == nil {
			out.Network = &modelpb.Network{}
		}
		if out.Network.Connection == nil {
			out.Network.Connection = &modelpb.NetworkConnection{}
		}
		out.Network.Connection.Type = m.Network.Connection.Type.Val
	}
}

func mapToTransactionMetricsetModel(from *transactionMetricset, event *modelpb.APMEvent) bool {
	event.Metricset = &modelpb.Metricset{}

	if from.Span.IsSet() {
		event.Span = &modelpb.Span{}
		if from.Span.Subtype.IsSet() {
			event.Span.Subtype = from.Span.Subtype.Val
		}
		if from.Span.Type.IsSet() {
			event.Span.Type = from.Span.Type.Val
		}
	}

	var ok bool
	if from.Samples.IsSet() {
		if event.Span != nil {
			if value := from.Samples.SpanSelfTimeCount.Value; value.IsSet() {
				if event.Span.SelfTime == nil {
					event.Span.SelfTime = &modelpb.AggregatedDuration{}
				}
				event.Span.SelfTime.Count = uint64(value.Val)
				ok = true
			}
			if value := from.Samples.SpanSelfTimeSum.Value; value.IsSet() {
				if event.Span.SelfTime == nil {
					event.Span.SelfTime = &modelpb.AggregatedDuration{}
				}
				event.Span.SelfTime.Sum = uint64(value.Val * 1000)
				ok = true
			}
		}
	}
	return ok
}

func mapToResponseModel(from contextResponse, out *modelpb.HTTPResponse) {
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Headers.Val, out.Headers)
	}
	if from.StatusCode.IsSet() {
		out.StatusCode = uint32(from.StatusCode.Val)
	}
	if from.TransferSize.IsSet() {
		val := uint64(from.TransferSize.Val)
		out.TransferSize = &val
	}
	if from.EncodedBodySize.IsSet() {
		val := uint64(from.EncodedBodySize.Val)
		out.EncodedBodySize = &val
	}
	if from.DecodedBodySize.IsSet() {
		val := uint64(from.DecodedBodySize.Val)
		out.DecodedBodySize = &val
	}
}

func mapToRequestModel(from contextRequest, out *modelpb.HTTPRequest) {
	if from.Method.IsSet() {
		out.Method = from.Method.Val
	}
	if len(from.Env) > 0 {
		out.Env = modeldecoderutil.ToKv(from.Env, out.Env)
	}
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Headers.Val, out.Headers)
	}
}

func mapToServiceModel(from contextService, out *modelpb.Service) {
	if from.Environment.IsSet() {
		out.Environment = from.Environment.Val
	}
	if from.Framework.IsSet() {
		if out.Framework == nil {
			out.Framework = &modelpb.Framework{}
		}
		if from.Framework.Name.IsSet() {
			out.Framework.Name = from.Framework.Name.Val
		}
		if from.Framework.Version.IsSet() {
			out.Framework.Version = from.Framework.Version.Val
		}
	}
	if from.Language.IsSet() {
		if out.Language == nil {
			out.Language = &modelpb.Language{}
		}
		if from.Language.Name.IsSet() {
			out.Language.Name = from.Language.Name.Val
		}
		if from.Language.Version.IsSet() {
			out.Language.Version = from.Language.Version.Val
		}
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
		out.Version = from.Version.Val
	}
	if from.Runtime.IsSet() {
		if out.Runtime == nil {
			out.Runtime = &modelpb.Runtime{}
		}
		if from.Runtime.Name.IsSet() {
			out.Runtime.Name = from.Runtime.Name.Val
		}
		if from.Runtime.Version.IsSet() {
			out.Runtime.Version = from.Runtime.Version.Val
		}
	}
}

func mapToAgentModel(from contextServiceAgent, out *modelpb.Agent) {
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Version.IsSet() {
		out.Version = from.Version.Val
	}
}

func mapToSpanModel(from *span, event *modelpb.APMEvent) {
	out := modelpb.Span{}
	out.Type = "unknown"
	event.Span = &out

	// map span specific data
	if !from.Action.IsSet() && !from.Subtype.IsSet() {
		sep := "."
		before, after, ok := strings.Cut(from.Type.Val, sep)
		if before != "" {
			out.Type = before
		}
		if ok {
			out.Subtype, out.Action, _ = strings.Cut(after, sep)
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
			if event.Destination == nil {
				event.Destination = &modelpb.Destination{}
			}
			event.Destination.Address = from.Context.Destination.Address.Val
		}
		if from.Context.Destination.Port.IsSet() {
			if event.Destination == nil {
				event.Destination = &modelpb.Destination{}
			}
			event.Destination.Port = uint32(from.Context.Destination.Port.Val)
		}
	}
	if from.Context.Destination.Service.IsSet() {
		service := modelpb.DestinationService{}
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
		var response *modelpb.HTTPResponse
		if from.Context.HTTP.Method.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			event.Http.Request = &modelpb.HTTPRequest{}
			event.Http.Request.Method = from.Context.HTTP.Method.Val
		}
		if from.Context.HTTP.StatusCode.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			if response == nil {
				response = &modelpb.HTTPResponse{}
			}
			event.Http.Response = response
			event.Http.Response.StatusCode = uint32(from.Context.HTTP.StatusCode.Val)
		}
		if from.Context.HTTP.URL.IsSet() {
			if event.Url == nil {
				event.Url = &modelpb.URL{}
			}
			event.Url.Original = from.Context.HTTP.URL.Val
		}
		if from.Context.HTTP.Response.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			if response == nil {
				response = &modelpb.HTTPResponse{}
			}
			event.Http.Response = response
			if from.Context.HTTP.Response.DecodedBodySize.IsSet() {
				val := uint64(from.Context.HTTP.Response.DecodedBodySize.Val)
				event.Http.Response.DecodedBodySize = &val
			}
			if from.Context.HTTP.Response.EncodedBodySize.IsSet() {
				val := uint64(from.Context.HTTP.Response.EncodedBodySize.Val)
				event.Http.Response.EncodedBodySize = &val
			}
			if from.Context.HTTP.Response.TransferSize.IsSet() {
				val := uint64(from.Context.HTTP.Response.TransferSize.Val)
				event.Http.Response.TransferSize = &val
			}
		}
	}
	if from.Context.Service.IsSet() {
		if from.Context.Service.Name.IsSet() {
			if event.Service == nil {
				event.Service = &modelpb.Service{}
			}
			event.Service.Name = from.Context.Service.Name.Val
		}
	}
	if len(from.Context.Tags) > 0 {
		modeldecoderutil.MergeLabels(from.Context.Tags, event)
	}
	if from.Duration.IsSet() {
		if event.Event == nil {
			event.Event = &modelpb.Event{}
		}
		event.Event.Duration = uint64(from.Duration.Val * float64(time.Millisecond))
	}
	if from.ID.IsSet() {
		out.Id = from.ID.Val
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if event.Event == nil {
		event.Event = &modelpb.Event{}
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
		out.Stacktrace = modeldecoderutil.ResliceAndPopulateNil(
			out.Stacktrace,
			len(from.Stacktrace),
			modelpb.StacktraceFrameFromVTPool,
		)
		mapToStracktraceModel(from.Stacktrace, out.Stacktrace)
	}
	if from.Sync.IsSet() {
		val := from.Sync.Val
		out.Sync = &val
	}
	if from.Start.IsSet() {
		// event.Timestamp is initialized to the time the payload was
		// received; offset that by "start" milliseconds for RUM.
		event.Timestamp += uint64(time.Duration(float64(time.Millisecond) * from.Start.Val).Nanoseconds())
	}
}

func mapToStracktraceModel(from []stacktraceFrame, out []*modelpb.StacktraceFrame) {
	for idx, eventFrame := range from {
		fr := out[idx]
		if eventFrame.AbsPath.IsSet() {
			fr.AbsPath = eventFrame.AbsPath.Val
		}
		if eventFrame.Classname.IsSet() {
			fr.Classname = eventFrame.Classname.Val
		}
		if eventFrame.ColumnNumber.IsSet() {
			val := uint32(eventFrame.ColumnNumber.Val)
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
			val := uint32(eventFrame.LineNumber.Val)
			fr.Lineno = &val
		}
		if eventFrame.Module.IsSet() {
			fr.Module = eventFrame.Module.Val
		}
		if len(eventFrame.PostContext) > 0 {
			fr.PostContext = modeldecoderutil.Reslice(fr.PostContext, len(eventFrame.PostContext))
			copy(fr.PostContext, eventFrame.PostContext)
		}
		if len(eventFrame.PreContext) > 0 {
			fr.PreContext = modeldecoderutil.Reslice(fr.PreContext, len(eventFrame.PreContext))
			copy(fr.PreContext, eventFrame.PreContext)
		}
	}
}

func mapToTransactionModel(from *transaction, event *modelpb.APMEvent) {
	out := modelpb.Transaction{}
	out.Type = "unknown"
	event.Transaction = &out

	// overwrite metadata with event specific information
	if from.Context.Service.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		mapToServiceModel(from.Context.Service, event.Service)
	}
	if from.Context.Service.Agent.IsSet() {
		if event.Agent == nil {
			event.Agent = &modelpb.Agent{}
		}
		mapToAgentModel(from.Context.Service.Agent, event.Agent)
	}
	overwriteUserInMetadataModel(from.Context.User, event)
	if from.Context.Request.Headers.IsSet() {
		if event.UserAgent == nil {
			event.UserAgent = &modelpb.UserAgent{}
		}
		mapToUserAgentModel(from.Context.Request.Headers, event.UserAgent)
	}

	// map transaction specific data
	if from.Context.IsSet() {
		if len(from.Context.Custom) > 0 {
			out.Custom = modeldecoderutil.ToKv(from.Context.Custom, out.Custom)
		}
		if len(from.Context.Tags) > 0 {
			modeldecoderutil.MergeLabels(from.Context.Tags, event)
		}
		if from.Context.Request.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			event.Http.Request = &modelpb.HTTPRequest{}
			mapToRequestModel(from.Context.Request, event.Http.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				event.Http.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Response.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			event.Http.Response = &modelpb.HTTPResponse{}
			mapToResponseModel(from.Context.Response, event.Http.Response)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() {
				event.Url = modelpb.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.Http == nil {
					event.Http = &modelpb.HTTP{}
				}
				if event.Http.Request == nil {
					event.Http.Request = &modelpb.HTTPRequest{}
				}
				event.Http.Request.Referrer = from.Context.Page.Referer.Val
			}
		}
	}
	if from.Duration.IsSet() {
		if event.Event == nil {
			event.Event = &modelpb.Event{}
		}
		event.Event.Duration = uint64(from.Duration.Val * float64(time.Millisecond))
	}
	if from.ID.IsSet() {
		out.Id = from.ID.Val
	}
	if from.Marks.IsSet() {
		out.Marks = make(map[string]*modelpb.TransactionMark, len(from.Marks.Events))
		for event, val := range from.Marks.Events {
			if len(val.Measurements) > 0 {
				tm := modelpb.TransactionMark{}
				tm.Measurements = val.Measurements
				out.Marks[event] = &tm
			}
		}
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if event.Event == nil {
		event.Event = &modelpb.Event{}
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
		event.ParentId = from.ParentID.Val
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
		event.Session = &modelpb.Session{}
		event.Session.Id = from.Session.ID.Val
		event.Session.Sequence = uint64(from.Session.Sequence.Val)
	}
	if from.SpanCount.Dropped.IsSet() {
		if out.SpanCount == nil {
			out.SpanCount = &modelpb.SpanCount{}
		}
		dropped := uint32(from.SpanCount.Dropped.Val)
		out.SpanCount.Dropped = &dropped
	}
	if from.SpanCount.Started.IsSet() {
		if out.SpanCount == nil {
			out.SpanCount = &modelpb.SpanCount{}
		}
		started := uint32(from.SpanCount.Started.Val)
		out.SpanCount.Started = &started
	}
	if from.TraceID.IsSet() {
		event.Trace = &modelpb.Trace{}
		event.Trace.Id = from.TraceID.Val
	}
	if from.Type.IsSet() {
		out.Type = from.Type.Val
	}
	if from.UserExperience.IsSet() {
		out.UserExperience = &modelpb.UserExperience{}
		out.UserExperience.CumulativeLayoutShift = -1
		out.UserExperience.FirstInputDelay = -1
		out.UserExperience.TotalBlockingTime = -1
		out.UserExperience.LongTask = nil
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
			out.UserExperience.LongTask = &modelpb.LongtaskMetrics{}
			out.UserExperience.LongTask.Count = uint64(from.UserExperience.Longtask.Count.Val)
			out.UserExperience.LongTask.Sum = from.UserExperience.Longtask.Sum.Val
			out.UserExperience.LongTask.Max = from.UserExperience.Longtask.Max.Val
		}
	}
}

func mapToUserAgentModel(from nullable.HTTPHeader, out *modelpb.UserAgent) {
	// overwrite userAgent information if available
	if h := from.Val.Values(textproto.CanonicalMIMEHeaderKey("User-Agent")); len(h) > 0 {
		out.Original = strings.Join(h, ", ")
	}
}

func overwriteUserInMetadataModel(from user, out *modelpb.APMEvent) {
	// overwrite User specific values if set
	// either populate all User fields or none to avoid mixing
	// different user data
	if !from.Domain.IsSet() && !from.ID.IsSet() && !from.Email.IsSet() && !from.Name.IsSet() {
		return
	}
	out.User = &modelpb.User{}
	if from.Domain.IsSet() {
		out.User.Domain = from.Domain.Val
	}
	if from.ID.IsSet() {
		out.User.Id = fmt.Sprint(from.ID.Val)
	}
	if from.Email.IsSet() {
		out.User.Email = from.Email.Val
	}
	if from.Name.IsSet() {
		out.User.Name = from.Name.Val
	}
}
