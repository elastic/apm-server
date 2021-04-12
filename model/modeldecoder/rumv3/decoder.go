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

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
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
	metricsetRootPool = sync.Pool{
		New: func() interface{} {
			return &metricsetRoot{}
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

func fetchMetricsetRoot() *metricsetRoot {
	return metricsetRootPool.Get().(*metricsetRoot)
}

func releaseMetricsetRoot(root *metricsetRoot) {
	root.Reset()
	metricsetRootPool.Put(root)
}

func fetchTransactionRoot() *transactionRoot {
	return transactionRootPool.Get().(*transactionRoot)
}

func releaseTransactionRoot(m *transactionRoot) {
	m.Reset()
	transactionRootPool.Put(m)
}

// DecodeNestedMetadata uses the given decoder to create the input models,
// then runs the defined validations on the input models
// and finally maps the values fom the input model to the given *model.Metadata instance
func DecodeNestedMetadata(d decoder.Decoder, out *model.Metadata) error {
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

// DecodeNestedError uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Error instance
//
// DecodeNestedError should be used when the stream in the decoder contains the `error` key
func DecodeNestedError(d decoder.Decoder, input *modeldecoder.Input, out *model.Error) error {
	root := fetchErrorRoot()
	defer releaseErrorRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToErrorModel(&root.Error, &input.Metadata, input.RequestTime, out)
	return nil
}

// DecodeNestedMetricset uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Metricset instance
//
// DecodeNestedMetricset should be used when the stream in the decoder contains the `metricset` key
func DecodeNestedMetricset(d decoder.Decoder, input *modeldecoder.Input, out *model.Metricset) error {
	root := fetchMetricsetRoot()
	defer releaseMetricsetRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToMetricsetModel(&root.Metricset, &input.Metadata, input.RequestTime, out)
	return nil
}

// Transaction is a wrapper around input models that can be nested inside a
// RUM v3 transaction
type Transaction struct {
	Transaction model.Transaction
	Metricsets  []*model.Metricset
	Spans       []*model.Span
}

// DecodeNestedTransaction uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Transaction instance
//
// DecodeNestedTransaction should be used when the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, out *Transaction) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	if err := d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToTransactionModel(&root.Transaction, &input.Metadata, input.RequestTime, &out.Transaction)
	for _, m := range root.Transaction.Metricsets {
		var outM model.Metricset
		mapToMetricsetModel(&m, &input.Metadata, input.RequestTime, &outM)
		outM.Transaction.Name = out.Transaction.Name
		outM.Transaction.Type = out.Transaction.Type
		out.Metricsets = append(out.Metricsets, &outM)
	}
	out.Spans = make([]*model.Span, len(root.Transaction.Spans))
	for idx, s := range root.Transaction.Spans {
		var outS model.Span
		mapToSpanModel(&s, &input.Metadata, input.RequestTime, &outS)
		outS.TransactionID = out.Transaction.ID
		outS.TraceID = out.Transaction.TraceID
		if s.ParentIndex.IsSet() && s.ParentIndex.Val >= 0 && s.ParentIndex.Val < idx {
			outS.ParentID = out.Spans[s.ParentIndex.Val].ID
		} else {
			outS.ParentID = out.Transaction.ID
		}
		out.Spans[idx] = &outS
	}
	return nil
}

func mapToErrorModel(from *errorEvent, metadata *model.Metadata, reqTime time.Time, out *model.Error) {
	// set metadata information
	if metadata != nil {
		out.Metadata = *metadata
	}
	if from == nil {
		return
	}
	// overwrite metadata with event specific information
	mapToServiceModel(from.Context.Service, &out.Metadata.Service)
	overwriteUserInMetadataModel(from.Context.User, &out.Metadata)
	mapToUserAgentModel(from.Context.Request.Headers, &out.Metadata)

	// map errorEvent specific data
	if from.Context.IsSet() {
		// metadata labels and context labels are merged only in the output model
		if len(from.Context.Tags) > 0 {
			out.Labels = from.Context.Tags.Clone()
		}
		if from.Context.Request.IsSet() {
			out.HTTP = &model.Http{Request: &model.Req{}}
			mapToRequestModel(from.Context.Request, out.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				out.HTTP.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Response.IsSet() {
			if out.HTTP == nil {
				out.HTTP = &model.Http{}
			}
			out.HTTP.Response = &model.Resp{}
			mapToResponseModel(from.Context.Response, out.HTTP.Response)
		}
		if from.Context.Page.IsSet() {
			out.Page = &model.Page{}
			mapToPageModel(from.Context.Page, out.Page)
			out.URL = out.Page.URL
			if out.Page.Referer != "" {
				if out.HTTP == nil {
					out.HTTP = &model.Http{}
				}
				if out.HTTP.Request == nil {
					out.HTTP.Request = &model.Req{}
				}
				out.HTTP.Request.Referer = out.Page.Referer
			}
		}
		if len(from.Context.Custom) > 0 {
			out.Custom = from.Context.Custom.Clone()
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
		out.ParentID = from.ParentID.Val
	}
	if from.Timestamp.Val.IsZero() {
		out.Timestamp = reqTime
	} else {
		out.Timestamp = from.Timestamp.Val
	}
	if from.TraceID.IsSet() {
		out.TraceID = from.TraceID.Val
	}
	if from.Transaction.Sampled.IsSet() {
		val := from.Transaction.Sampled.Val
		out.TransactionSampled = &val
	}
	if from.Transaction.Type.IsSet() {
		out.TransactionType = from.Transaction.Type.Val
	}
	if from.TransactionID.IsSet() {
		out.TransactionID = from.TransactionID.Val
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
		out.Code = from.Code.Val
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

func mapToMetadataModel(m *metadata, out *model.Metadata) {
	// Labels
	if len(m.Labels) > 0 {
		out.Labels = common.MapStr{}
		out.Labels.Update(m.Labels)
	}

	// Service
	if m.Service.Agent.Name.IsSet() {
		out.Service.Agent.Name = m.Service.Agent.Name.Val
	}
	if m.Service.Agent.Version.IsSet() {
		out.Service.Agent.Version = m.Service.Agent.Version.Val
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
	if m.User.ID.IsSet() {
		out.User.ID = fmt.Sprint(m.User.ID.Val)
	}
	if m.User.Email.IsSet() {
		out.User.Email = m.User.Email.Val
	}
	if m.User.Name.IsSet() {
		out.User.Name = m.User.Name.Val
	}
}

func mapToMetricsetModel(from *metricset, metadata *model.Metadata, reqTime time.Time, out *model.Metricset) {
	// set metadata as they are - no values are overwritten by the event
	if metadata != nil {
		out.Metadata = *metadata
	}
	if from == nil {
		return
	}
	// set timestamp from requst time
	out.Timestamp = reqTime

	// map samples information
	if from.Samples.IsSet() {
		if from.Samples.TransactionDurationCount.Value.IsSet() {
			s := model.Sample{Name: metricsetSamplesTransactionDurationCountName, Value: from.Samples.TransactionDurationCount.Value.Val}
			out.Samples = append(out.Samples, s)
		}
		if from.Samples.TransactionDurationSum.Value.IsSet() {
			s := model.Sample{Name: metricsetSamplesTransactionDurationSumName, Value: from.Samples.TransactionDurationSum.Value.Val}
			out.Samples = append(out.Samples, s)
		}
		if from.Samples.TransactionBreakdownCount.Value.IsSet() {
			s := model.Sample{Name: metricsetSamplesTransactionBreakdownCountName, Value: from.Samples.TransactionBreakdownCount.Value.Val}
			out.Samples = append(out.Samples, s)
		}
		if from.Samples.SpanSelfTimeCount.Value.IsSet() {
			s := model.Sample{Name: metricsetSamplesSpanSelfTimeCountName, Value: from.Samples.SpanSelfTimeCount.Value.Val}
			out.Samples = append(out.Samples, s)
		}
		if from.Samples.SpanSelfTimeSum.Value.IsSet() {
			s := model.Sample{Name: metricsetSamplesSpanSelfTimeSumName, Value: from.Samples.SpanSelfTimeSum.Value.Val}
			out.Samples = append(out.Samples, s)
		}
	}

	if len(from.Tags) > 0 {
		out.Labels = from.Tags.Clone()
	}
	// map span information
	if from.Span.Subtype.IsSet() {
		out.Span.Subtype = from.Span.Subtype.Val
	}
	if from.Span.Type.IsSet() {
		out.Span.Type = from.Span.Type.Val
	}
}

func mapToPageModel(from contextPage, out *model.Page) {
	if from.URL.IsSet() {
		out.URL = model.ParseURL(from.URL.Val, "", "")
	}
	if from.Referer.IsSet() {
		out.Referer = from.Referer.Val
	}
}

func mapToResponseModel(from contextResponse, out *model.Resp) {
	if from.Headers.IsSet() {
		out.Headers = from.Headers.Val.Clone()
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

func mapToRequestModel(from contextRequest, out *model.Req) {
	if from.Method.IsSet() {
		out.Method = from.Method.Val
	}
	if len(from.Env) > 0 {
		out.Env = from.Env.Clone()
	}
	if from.Headers.IsSet() {
		out.Headers = from.Headers.Val.Clone()
	}
}

func mapToServiceModel(from contextService, out *model.Service) {
	if from.Agent.Name.IsSet() {
		out.Agent.Name = from.Agent.Name.Val
	}
	if from.Agent.Version.IsSet() {
		out.Agent.Version = from.Agent.Version.Val
	}
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

func mapToSpanModel(from *span, metadata *model.Metadata, reqTime time.Time, out *model.Span) {
	// set metadata information for span
	if metadata != nil {
		out.Metadata = *metadata
	}
	if from == nil {
		return
	}
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
		destination := model.Destination{}
		if from.Context.Destination.Address.IsSet() {
			destination.Address = from.Context.Destination.Address.Val
		}
		if from.Context.Destination.Port.IsSet() {
			destination.Port = from.Context.Destination.Port.Val
		}
		out.Destination = &destination
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
		http := model.HTTP{}
		if from.Context.HTTP.Method.IsSet() {
			http.Method = from.Context.HTTP.Method.Val
		}
		if from.Context.HTTP.StatusCode.IsSet() {
			http.StatusCode = from.Context.HTTP.StatusCode.Val
		}
		if from.Context.HTTP.URL.IsSet() {
			http.URL = from.Context.HTTP.URL.Val
		}
		if from.Context.HTTP.Response.IsSet() {
			http.Response = &model.MinimalResp{}
			if from.Context.HTTP.Response.DecodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.DecodedBodySize.Val
				http.Response.DecodedBodySize = &val
			}
			if from.Context.HTTP.Response.EncodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.EncodedBodySize.Val
				http.Response.EncodedBodySize = &val
			}
			if from.Context.HTTP.Response.TransferSize.IsSet() {
				val := from.Context.HTTP.Response.TransferSize.Val
				http.Response.TransferSize = &val
			}
		}
		out.HTTP = &http
	}
	if from.Context.Service.IsSet() {
		if from.Context.Service.Agent.Name.IsSet() {
			out.Metadata.Service.Agent.Name = from.Context.Service.Agent.Name.Val
		}
		if from.Context.Service.Agent.Version.IsSet() {
			out.Metadata.Service.Agent.Version = from.Context.Service.Agent.Version.Val
		}
		if from.Context.Service.Name.IsSet() {
			out.Metadata.Service.Name = from.Context.Service.Name.Val
		}
	}
	if len(from.Context.Tags) > 0 {
		out.Labels = from.Context.Tags.Clone()
	}
	if from.Duration.IsSet() {
		out.Duration = from.Duration.Val
	}
	if from.ID.IsSet() {
		out.ID = from.ID.Val
	}
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Outcome.IsSet() {
		out.Outcome = from.Outcome.Val
	} else {
		if from.Context.HTTP.StatusCode.IsSet() {
			statusCode := from.Context.HTTP.StatusCode.Val
			if statusCode >= http.StatusBadRequest {
				out.Outcome = "failure"
			} else {
				out.Outcome = "success"
			}
		} else {
			out.Outcome = "unknown"
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
		// adjust timestamp to be reqTime + start
		reqTime = reqTime.Add(time.Duration(float64(time.Millisecond) * from.Start.Val))
	}
	out.Timestamp = reqTime
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

func mapToTransactionModel(from *transaction, metadata *model.Metadata, reqTime time.Time, out *model.Transaction) {
	// set metadata information
	if metadata != nil {
		out.Metadata = *metadata
	}
	if from == nil {
		return
	}
	// overwrite metadata with event specific information
	mapToServiceModel(from.Context.Service, &out.Metadata.Service)
	overwriteUserInMetadataModel(from.Context.User, &out.Metadata)
	mapToUserAgentModel(from.Context.Request.Headers, &out.Metadata)

	// map transaction specific data

	if from.Context.IsSet() {
		if len(from.Context.Custom) > 0 {
			out.Custom = from.Context.Custom.Clone()
		}
		// metadata labels and context labels are merged when transforming the output model
		if len(from.Context.Tags) > 0 {
			out.Labels = from.Context.Tags.Clone()
		}
		if from.Context.Request.IsSet() {
			out.HTTP = &model.Http{Request: &model.Req{}}
			mapToRequestModel(from.Context.Request, out.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				out.HTTP.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Response.IsSet() {
			if out.HTTP == nil {
				out.HTTP = &model.Http{}
			}
			out.HTTP.Response = &model.Resp{}
			mapToResponseModel(from.Context.Response, out.HTTP.Response)
		}
		if from.Context.Page.IsSet() {
			out.Page = &model.Page{}
			mapToPageModel(from.Context.Page, out.Page)
			out.URL = out.Page.URL
			if out.Page.Referer != "" {
				if out.HTTP == nil {
					out.HTTP = &model.Http{}
				}
				if out.HTTP.Request == nil {
					out.HTTP.Request = &model.Req{}
				}
				out.HTTP.Request.Referer = out.Page.Referer
			}
		}
	}
	if from.Duration.IsSet() {
		out.Duration = from.Duration.Val
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
		out.Outcome = from.Outcome.Val
	} else {
		if from.Context.Response.StatusCode.IsSet() {
			statusCode := from.Context.Response.StatusCode.Val
			if statusCode >= http.StatusInternalServerError {
				out.Outcome = "failure"
			} else {
				out.Outcome = "success"
			}
		} else {
			out.Outcome = "unknown"
		}
	}
	if from.ParentID.IsSet() {
		out.ParentID = from.ParentID.Val
	}
	if from.Result.IsSet() {
		out.Result = from.Result.Val
	}

	sampled := true
	if from.Sampled.IsSet() {
		sampled = from.Sampled.Val
	}
	out.Sampled = &sampled
	if from.SampleRate.IsSet() {
		if from.SampleRate.Val > 0 {
			out.RepresentativeCount = 1 / from.SampleRate.Val
		}
	} else {
		out.RepresentativeCount = 1
	}
	if from.Session.ID.IsSet() {
		out.Session.ID = from.Session.ID.Val
		out.Session.Sequence = from.Session.Sequence.Val
	}
	if from.SpanCount.Dropped.IsSet() {
		dropped := from.SpanCount.Dropped.Val
		out.SpanCount.Dropped = &dropped
	}
	if from.SpanCount.Started.IsSet() {
		started := from.SpanCount.Started.Val
		out.SpanCount.Started = &started
	}
	out.Timestamp = reqTime
	if from.TraceID.IsSet() {
		out.TraceID = from.TraceID.Val
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

func mapToUserAgentModel(from nullable.HTTPHeader, out *model.Metadata) {
	// overwrite userAgent information if available
	if from.IsSet() {
		if h := from.Val.Values(textproto.CanonicalMIMEHeaderKey("User-Agent")); len(h) > 0 {
			out.UserAgent.Original = strings.Join(h, ", ")
		}
	}
}

func overwriteUserInMetadataModel(from user, out *model.Metadata) {
	// overwrite User specific values if set
	// either populate all User fields or none to avoid mixing
	// different user data
	if !from.ID.IsSet() && !from.Email.IsSet() && !from.Name.IsSet() {
		return
	}
	out.User = model.User{}
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
