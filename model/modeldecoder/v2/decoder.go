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
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	"github.com/elastic/apm-server/utility"
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
	spanRootPool = sync.Pool{
		New: func() interface{} {
			return &spanRoot{}
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

func releaseMetadataRoot(root *metadataRoot) {
	root.Reset()
	metadataRootPool.Put(root)
}

func fetchMetricsetRoot() *metricsetRoot {
	return metricsetRootPool.Get().(*metricsetRoot)
}

func releaseMetricsetRoot(root *metricsetRoot) {
	root.Reset()
	metricsetRootPool.Put(root)
}

func fetchSpanRoot() *spanRoot {
	return spanRootPool.Get().(*spanRoot)
}

func releaseSpanRoot(root *spanRoot) {
	root.Reset()
	spanRootPool.Put(root)
}

func fetchTransactionRoot() *transactionRoot {
	return transactionRootPool.Get().(*transactionRoot)
}

func releaseTransactionRoot(root *transactionRoot) {
	root.Reset()
	transactionRootPool.Put(root)
}

// DecodeMetadata uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Metadata instance
//
// DecodeMetadata should be used when the the stream in the decoder does not contain the
// `metadata` key, but only the metadata data.
func DecodeMetadata(d decoder.Decoder, out *model.Metadata) error {
	return decodeMetadata(decodeIntoMetadata, d, out)
}

// DecodeNestedMetadata uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Metadata instance
//
// DecodeNestedMetadata should be used when the stream in the decoder contains the `metadata` key
func DecodeNestedMetadata(d decoder.Decoder, out *model.Metadata) error {
	return decodeMetadata(decodeIntoMetadataRoot, d, out)
}

// DecodeNestedError uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Error instance
//
// DecodeNestedError should be used when the stream in the decoder contains the `error` key
func DecodeNestedError(d decoder.Decoder, input *modeldecoder.Input, out *model.Error) error {
	root := fetchErrorRoot()
	defer releaseErrorRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToErrorModel(&root.Error, &input.Metadata, input.RequestTime, input.Config, out)
	return err
}

// DecodeNestedMetricset uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Metricset instance
//
// DecodeNestedMetricset should be used when the stream in the decoder contains the `metricset` key
func DecodeNestedMetricset(d decoder.Decoder, input *modeldecoder.Input, out *model.Metricset) error {
	root := fetchMetricsetRoot()
	defer releaseMetricsetRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToMetricsetModel(&root.Metricset, &input.Metadata, input.RequestTime, input.Config, out)
	return err
}

// DecodeNestedSpan uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Span instance
//
// DecodeNestedSpan should be used when the stream in the decoder contains the `span` key
func DecodeNestedSpan(d decoder.Decoder, input *modeldecoder.Input, out *model.Span) error {
	root := fetchSpanRoot()
	defer releaseSpanRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToSpanModel(&root.Span, &input.Metadata, input.RequestTime, input.Config, out)
	return err
}

// DecodeNestedTransaction uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Transaction instance
//
// DecodeNestedTransaction should be used when the stream in the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, out *model.Transaction) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToTransactionModel(&root.Transaction, &input.Metadata, input.RequestTime, input.Config, out)
	return err
}

func decodeMetadata(decFn func(d decoder.Decoder, m *metadataRoot) error, d decoder.Decoder, out *model.Metadata) error {
	m := fetchMetadataRoot()
	defer releaseMetadataRoot(m)
	var err error
	if err = decFn(d, m); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := m.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	mapToMetadataModel(&m.Metadata, out)
	return err
}

func decodeIntoMetadata(d decoder.Decoder, m *metadataRoot) error {
	return d.Decode(&m.Metadata)
}

func decodeIntoMetadataRoot(d decoder.Decoder, m *metadataRoot) error {
	return d.Decode(m)
}

func mapToClientModel(from contextRequest, out *model.Metadata) {
	// only set client information if not already set in metadata
	// this is aligned with current logic
	if out.Client.IP != nil {
		return
	}
	// http.Request.Headers and http.Request.Socket information is
	// only set for backend events; try to first extract an IP address
	// from the headers, if not possible use IP address from socket remote_address
	if ip := utility.ExtractIPFromHeader(from.Headers.Val); ip != nil {
		out.Client.IP = ip
	} else if ip := utility.ParseIP(from.Socket.RemoteAddress.Val); ip != nil {
		out.Client.IP = ip
	}
}

func mapToErrorModel(from *errorEvent, metadata *model.Metadata, reqTime time.Time, config modeldecoder.Config, out *model.Error) {
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
	mapToClientModel(from.Context.Request, &out.Metadata)

	// map errorEvent specific data

	if from.Context.IsSet() {
		if config.Experimental && from.Context.Experimental.IsSet() {
			out.Experimental = from.Context.Experimental.Val
		}
		// metadata labels and context labels are merged only in the output model
		if len(from.Context.Tags) > 0 {
			out.Labels = from.Context.Tags.Clone()
		}
		if from.Context.Page.IsSet() {
			out.Page = &model.Page{}
			mapToPageModel(from.Context.Page, out.Page)
		}
		if from.Context.Request.IsSet() {
			out.HTTP = &model.Http{Request: &model.Req{}}
			mapToRequestModel(from.Context.Request, out.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				val := from.Context.Request.HTTPVersion.Val
				out.HTTP.Version = &val
			}
		}
		if from.Context.Response.IsSet() {
			if out.HTTP == nil {
				out.HTTP = &model.Http{}
			}
			out.HTTP.Response = &model.Resp{}
			mapToResponseModel(from.Context.Response, out.HTTP.Response)
		}
		if from.Context.Request.URL.IsSet() {
			out.URL = &model.URL{}
			mapToRequestURLModel(from.Context.Request.URL, out.URL)
		}
		if len(from.Context.Custom) > 0 {
			out.Custom = from.Context.Custom.Clone()
		}
	}
	if from.Culprit.IsSet() {
		val := from.Culprit.Val
		out.Culprit = &val
	}
	if from.Exception.IsSet() {
		out.Exception = &model.Exception{}
		mapToExceptionModel(from.Exception, out.Exception)
	}
	if from.ID.IsSet() {
		val := from.ID.Val
		out.ID = &val
	}
	if from.Log.IsSet() {
		log := model.Log{}
		if from.Log.Level.IsSet() {
			val := from.Log.Level.Val
			log.Level = &val
		}
		if from.Log.LoggerName.IsSet() {
			val := from.Log.LoggerName.Val
			log.LoggerName = &val
		}
		if from.Log.Message.IsSet() {
			log.Message = from.Log.Message.Val
		}
		if from.Log.ParamMessage.IsSet() {
			val := from.Log.ParamMessage.Val
			log.ParamMessage = &val
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
		val := from.Transaction.Type.Val
		out.TransactionType = &val
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
		out.Message = &from.Message.Val
	}
	if from.Module.IsSet() {
		out.Module = &from.Module.Val
	}
	if len(from.Stacktrace) > 0 {
		out.Stacktrace = make(model.Stacktrace, len(from.Stacktrace))
		mapToStracktraceModel(from.Stacktrace, out.Stacktrace)
	}
	if from.Type.IsSet() {
		out.Type = &from.Type.Val
	}
}

func mapToMetadataModel(from *metadata, out *model.Metadata) {
	// Cloud
	if from == nil || out == nil {
		return
	}
	if from.Cloud.Account.ID.IsSet() {
		out.Cloud.AccountID = from.Cloud.Account.ID.Val
	}
	if from.Cloud.Account.Name.IsSet() {
		out.Cloud.AccountName = from.Cloud.Account.Name.Val
	}
	if from.Cloud.AvailabilityZone.IsSet() {
		out.Cloud.AvailabilityZone = from.Cloud.AvailabilityZone.Val
	}
	if from.Cloud.Instance.ID.IsSet() {
		out.Cloud.InstanceID = from.Cloud.Instance.ID.Val
	}
	if from.Cloud.Instance.Name.IsSet() {
		out.Cloud.InstanceName = from.Cloud.Instance.Name.Val
	}
	if from.Cloud.Machine.Type.IsSet() {
		out.Cloud.MachineType = from.Cloud.Machine.Type.Val
	}
	if from.Cloud.Project.ID.IsSet() {
		out.Cloud.ProjectID = from.Cloud.Project.ID.Val
	}
	if from.Cloud.Project.Name.IsSet() {
		out.Cloud.ProjectName = from.Cloud.Project.Name.Val
	}
	if from.Cloud.Provider.IsSet() {
		out.Cloud.Provider = from.Cloud.Provider.Val
	}
	if from.Cloud.Region.IsSet() {
		out.Cloud.Region = from.Cloud.Region.Val
	}

	// Labels
	if len(from.Labels) > 0 {
		out.Labels = from.Labels.Clone()
	}

	// Process
	if len(from.Process.Argv) > 0 {
		out.Process.Argv = make([]string, len(from.Process.Argv))
		copy(out.Process.Argv, from.Process.Argv)
	}
	if from.Process.Pid.IsSet() {
		out.Process.Pid = from.Process.Pid.Val
	}
	if from.Process.Ppid.IsSet() {
		var pid = from.Process.Ppid.Val
		out.Process.Ppid = &pid
	}
	if from.Process.Title.IsSet() {
		out.Process.Title = from.Process.Title.Val
	}

	// Service
	if from.Service.Agent.EphemeralID.IsSet() {
		out.Service.Agent.EphemeralID = from.Service.Agent.EphemeralID.Val
	}
	if from.Service.Agent.Name.IsSet() {
		out.Service.Agent.Name = from.Service.Agent.Name.Val
	}
	if from.Service.Agent.Version.IsSet() {
		out.Service.Agent.Version = from.Service.Agent.Version.Val
	}
	if from.Service.Environment.IsSet() {
		out.Service.Environment = from.Service.Environment.Val
	}
	if from.Service.Framework.Name.IsSet() {
		out.Service.Framework.Name = from.Service.Framework.Name.Val
	}
	if from.Service.Framework.Version.IsSet() {
		out.Service.Framework.Version = from.Service.Framework.Version.Val
	}
	if from.Service.Language.Name.IsSet() {
		out.Service.Language.Name = from.Service.Language.Name.Val
	}
	if from.Service.Language.Version.IsSet() {
		out.Service.Language.Version = from.Service.Language.Version.Val
	}
	if from.Service.Name.IsSet() {
		out.Service.Name = from.Service.Name.Val
	}
	if from.Service.Node.Name.IsSet() {
		out.Service.Node.Name = from.Service.Node.Name.Val
	}
	if from.Service.Runtime.Name.IsSet() {
		out.Service.Runtime.Name = from.Service.Runtime.Name.Val
	}
	if from.Service.Runtime.Version.IsSet() {
		out.Service.Runtime.Version = from.Service.Runtime.Version.Val
	}
	if from.Service.Version.IsSet() {
		out.Service.Version = from.Service.Version.Val
	}

	// System
	if from.System.Architecture.IsSet() {
		out.System.Architecture = from.System.Architecture.Val
	}
	if from.System.ConfiguredHostname.IsSet() {
		out.System.ConfiguredHostname = from.System.ConfiguredHostname.Val
	}
	if from.System.Container.ID.IsSet() {
		out.System.Container.ID = from.System.Container.ID.Val
	}
	if from.System.DetectedHostname.IsSet() {
		out.System.DetectedHostname = from.System.DetectedHostname.Val
	}
	if !from.System.ConfiguredHostname.IsSet() && !from.System.DetectedHostname.IsSet() &&
		from.System.DeprecatedHostname.IsSet() {
		out.System.DetectedHostname = from.System.DeprecatedHostname.Val
	}
	if from.System.Kubernetes.Namespace.IsSet() {
		out.System.Kubernetes.Namespace = from.System.Kubernetes.Namespace.Val
	}
	if from.System.Kubernetes.Node.Name.IsSet() {
		out.System.Kubernetes.NodeName = from.System.Kubernetes.Node.Name.Val
	}
	if from.System.Kubernetes.Pod.Name.IsSet() {
		out.System.Kubernetes.PodName = from.System.Kubernetes.Pod.Name.Val
	}
	if from.System.Kubernetes.Pod.UID.IsSet() {
		out.System.Kubernetes.PodUID = from.System.Kubernetes.Pod.UID.Val
	}
	if from.System.Platform.IsSet() {
		out.System.Platform = from.System.Platform.Val
	}

	// User
	if from.User.ID.IsSet() {
		out.User.ID = fmt.Sprint(from.User.ID.Val)
	}
	if from.User.Email.IsSet() {
		out.User.Email = from.User.Email.Val
	}
	if from.User.Name.IsSet() {
		out.User.Name = from.User.Name.Val
	}
}

func mapToMetricsetModel(from *metricset, metadata *model.Metadata, reqTime time.Time, config modeldecoder.Config, out *model.Metricset) {
	// set metadata as they are - no values are overwritten by the event
	if metadata != nil {
		out.Metadata = *metadata
	}
	if from == nil {
		return
	}
	// set timestamp from input or requst time
	if from.Timestamp.Val.IsZero() {
		out.Timestamp = reqTime
	} else {
		out.Timestamp = from.Timestamp.Val
	}

	// map samples information
	if len(from.Samples) > 0 {
		out.Samples = make([]model.Sample, len(from.Samples))
		i := 0
		for name, sample := range from.Samples {
			out.Samples[i] = model.Sample{Name: name, Value: sample.Value.Val}
			i++
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
	// map transaction information
	if from.Transaction.Name.IsSet() {
		out.Transaction.Name = from.Transaction.Name.Val
	}
	if from.Transaction.Type.IsSet() {
		out.Transaction.Type = from.Transaction.Type.Val
	}
}

func mapToPageModel(from contextPage, out *model.Page) {
	if from.URL.IsSet() {
		out.URL = model.ParseURL(from.URL.Val, "")
	}
	if from.Referer.IsSet() {
		referer := from.Referer.Val
		out.Referer = &referer
	}
}

func mapToRequestModel(from contextRequest, out *model.Req) {
	if from.Method.IsSet() {
		out.Method = from.Method.Val
	}
	if len(from.Env) > 0 {
		out.Env = from.Env.Clone()
	}
	if from.Socket.IsSet() {
		out.Socket = &model.Socket{}
		if from.Socket.Encrypted.IsSet() {
			val := from.Socket.Encrypted.Val
			out.Socket.Encrypted = &val
		}
		if from.Socket.RemoteAddress.IsSet() {
			val := from.Socket.RemoteAddress.Val
			out.Socket.RemoteAddress = &val
		}
	}
	if from.Body.IsSet() {
		out.Body = from.Body.Val
	}
	if len(from.Cookies) > 0 {
		out.Cookies = from.Cookies.Clone()
	}
	if from.Headers.IsSet() {
		out.Headers = from.Headers.Val.Clone()
	}
}

func mapToRequestURLModel(from contextRequestURL, out *model.URL) {
	if from.Raw.IsSet() {
		val := from.Raw.Val
		out.Original = &val
	}
	if from.Full.IsSet() {
		val := from.Full.Val
		out.Full = &val
	}
	if from.Hostname.IsSet() {
		val := from.Hostname.Val
		out.Domain = &val
	}
	if from.Path.IsSet() {
		val := from.Path.Val
		out.Path = &val
	}
	if from.Search.IsSet() {
		val := from.Search.Val
		out.Query = &val
	}
	if from.Hash.IsSet() {
		val := from.Hash.Val
		out.Fragment = &val
	}
	if from.Protocol.IsSet() {
		trimmed := strings.TrimSuffix(from.Protocol.Val, ":")
		out.Scheme = &trimmed
	}
	if from.Port.IsSet() {
		// should never result in an error, type is checked when decoding
		port, err := strconv.Atoi(fmt.Sprint(from.Port.Val))
		if err == nil {
			out.Port = &port
		}
	}
}

func mapToResponseModel(from contextResponse, out *model.Resp) {
	if from.Finished.IsSet() {
		val := from.Finished.Val
		out.Finished = &val
	}
	if from.Headers.IsSet() {
		out.Headers = from.Headers.Val.Clone()
	}
	if from.HeadersSent.IsSet() {
		val := from.HeadersSent.Val
		out.HeadersSent = &val
	}
	if from.StatusCode.IsSet() {
		val := from.StatusCode.Val
		out.StatusCode = &val
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

func mapToServiceModel(from contextService, out *model.Service) {
	if from.Agent.EphemeralID.IsSet() {
		out.Agent.EphemeralID = from.Agent.EphemeralID.Val
	}
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
	if from.Node.Name.IsSet() {
		out.Node.Name = from.Node.Name.Val
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

func mapToSpanModel(from *span, metadata *model.Metadata, reqTime time.Time, config modeldecoder.Config, out *model.Span) {
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
			out.Subtype = &typ[1]
			if len(typ) > 2 {
				action := strings.Join(typ[2:], sep)
				out.Action = &action
			}
		}
	} else {
		if from.Action.IsSet() {
			val := from.Action.Val
			out.Action = &val
		}
		if from.Subtype.IsSet() {
			val := from.Subtype.Val
			out.Subtype = &val
		}
		if from.Type.IsSet() {
			out.Type = from.Type.Val
		}
	}
	if len(from.ChildIDs) > 0 {
		out.ChildIDs = make([]string, len(from.ChildIDs))
		copy(out.ChildIDs, from.ChildIDs)
	}
	if from.Context.Database.IsSet() {
		db := model.DB{}
		if from.Context.Database.Instance.IsSet() {
			val := from.Context.Database.Instance.Val
			db.Instance = &val
		}
		if from.Context.Database.Link.IsSet() {
			val := from.Context.Database.Link.Val
			db.Link = &val
		}
		if from.Context.Database.RowsAffected.IsSet() {
			val := from.Context.Database.RowsAffected.Val
			db.RowsAffected = &val
		}
		if from.Context.Database.Statement.IsSet() {
			val := from.Context.Database.Statement.Val
			db.Statement = &val
		}
		if from.Context.Database.Type.IsSet() {
			val := from.Context.Database.Type.Val
			db.Type = &val
		}
		if from.Context.Database.User.IsSet() {
			val := from.Context.Database.User.Val
			db.UserName = &val
		}
		out.DB = &db
	}
	if from.Context.Destination.Address.IsSet() || from.Context.Destination.Port.IsSet() {
		destination := model.Destination{}
		if from.Context.Destination.Address.IsSet() {
			val := from.Context.Destination.Address.Val
			destination.Address = &val
		}
		if from.Context.Destination.Port.IsSet() {
			val := from.Context.Destination.Port.Val
			destination.Port = &val
		}
		out.Destination = &destination
	}
	if from.Context.Destination.Service.IsSet() {
		service := model.DestinationService{}
		if from.Context.Destination.Service.Name.IsSet() {
			val := from.Context.Destination.Service.Name.Val
			service.Name = &val
		}
		if from.Context.Destination.Service.Resource.IsSet() {
			val := from.Context.Destination.Service.Resource.Val
			service.Resource = &val
		}
		if from.Context.Destination.Service.Type.IsSet() {
			val := from.Context.Destination.Service.Type.Val
			service.Type = &val
		}
		out.DestinationService = &service
	}
	if config.Experimental && from.Context.Experimental.IsSet() {
		out.Experimental = from.Context.Experimental.Val
	}
	if from.Context.HTTP.IsSet() {
		http := model.HTTP{}
		if from.Context.HTTP.Method.IsSet() {
			val := from.Context.HTTP.Method.Val
			http.Method = &val
		}
		if from.Context.HTTP.Response.IsSet() {
			response := model.MinimalResp{}
			if from.Context.HTTP.Response.DecodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.DecodedBodySize.Val
				response.DecodedBodySize = &val
			}
			if from.Context.HTTP.Response.EncodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.EncodedBodySize.Val
				response.EncodedBodySize = &val
			}
			if from.Context.HTTP.Response.Headers.IsSet() {
				response.Headers = from.Context.HTTP.Response.Headers.Val.Clone()
			}
			if from.Context.HTTP.Response.StatusCode.IsSet() {
				val := from.Context.HTTP.Response.StatusCode.Val
				response.StatusCode = &val
			}
			if from.Context.HTTP.Response.TransferSize.IsSet() {
				val := from.Context.HTTP.Response.TransferSize.Val
				response.TransferSize = &val
			}
			http.Response = &response
		}
		if from.Context.HTTP.StatusCode.IsSet() {
			val := from.Context.HTTP.StatusCode.Val
			http.StatusCode = &val
		}
		if from.Context.HTTP.URL.IsSet() {
			val := from.Context.HTTP.URL.Val
			http.URL = &val
		}
		out.HTTP = &http
	}
	if from.Context.Message.IsSet() {
		message := model.Message{}
		if from.Context.Message.Body.IsSet() {
			val := from.Context.Message.Body.Val
			message.Body = &val
		}
		if from.Context.Message.Headers.IsSet() {
			message.Headers = from.Context.Message.Headers.Val.Clone()
		}
		if from.Context.Message.Age.Milliseconds.IsSet() {
			val := from.Context.Message.Age.Milliseconds.Val
			message.AgeMillis = &val
		}
		if from.Context.Message.Queue.Name.IsSet() {
			val := from.Context.Message.Queue.Name.Val
			message.QueueName = &val
		}
		out.Message = &message
	}
	if from.Context.Service.IsSet() {
		out.Service = &model.Service{}
		mapToServiceModel(from.Context.Service, out.Service)
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
	if from.ParentID.IsSet() {
		out.ParentID = from.ParentID.Val
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
	if from.Timestamp.IsSet() && !from.Timestamp.Val.IsZero() {
		out.Timestamp = from.Timestamp.Val
	} else {
		timestamp := reqTime
		if from.Start.IsSet() {
			// adjust timestamp to be reqTime + start
			timestamp = timestamp.Add(time.Duration(float64(time.Millisecond) * from.Start.Val))
		}
		out.Timestamp = timestamp
	}
	if from.TraceID.IsSet() {
		out.TraceID = from.TraceID.Val
	}
	if from.TransactionID.IsSet() {
		out.TransactionID = from.TransactionID.Val
	}
}

func mapToStracktraceModel(from []stacktraceFrame, out model.Stacktrace) {
	for idx, eventFrame := range from {
		fr := model.StacktraceFrame{}
		if eventFrame.AbsPath.IsSet() {
			val := eventFrame.AbsPath.Val
			fr.AbsPath = &val
		}
		if eventFrame.Classname.IsSet() {
			val := eventFrame.Classname.Val
			fr.Classname = &val
		}
		if eventFrame.ColumnNumber.IsSet() {
			val := eventFrame.ColumnNumber.Val
			fr.Colno = &val
		}
		if eventFrame.ContextLine.IsSet() {
			val := eventFrame.ContextLine.Val
			fr.ContextLine = &val
		}
		if eventFrame.Filename.IsSet() {
			val := eventFrame.Filename.Val
			fr.Filename = &val
		}
		if eventFrame.Function.IsSet() {
			val := eventFrame.Function.Val
			fr.Function = &val
		}
		if eventFrame.LibraryFrame.IsSet() {
			val := eventFrame.LibraryFrame.Val
			fr.LibraryFrame = &val
		}
		if eventFrame.LineNumber.IsSet() {
			val := eventFrame.LineNumber.Val
			fr.Lineno = &val
		}
		if eventFrame.Module.IsSet() {
			val := eventFrame.Module.Val
			fr.Module = &val
		}
		if len(eventFrame.PostContext) > 0 {
			fr.PostContext = make([]string, len(eventFrame.PostContext))
			copy(fr.PostContext, eventFrame.PostContext)
		}
		if len(eventFrame.PreContext) > 0 {
			fr.PreContext = make([]string, len(eventFrame.PreContext))
			copy(fr.PreContext, eventFrame.PreContext)
		}
		if len(eventFrame.Vars) > 0 {
			fr.Vars = eventFrame.Vars.Clone()
		}
		out[idx] = &fr
	}
}

func mapToTransactionModel(from *transaction, metadata *model.Metadata, reqTime time.Time, config modeldecoder.Config, out *model.Transaction) {
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
	mapToClientModel(from.Context.Request, &out.Metadata)

	// map transaction specific data

	if from.Context.IsSet() {
		if len(from.Context.Custom) > 0 {
			out.Custom = from.Context.Custom.Clone()
		}
		if config.Experimental && from.Context.Experimental.IsSet() {
			out.Experimental = from.Context.Experimental.Val
		}
		// metadata labels and context labels are merged when transforming the output model
		if len(from.Context.Tags) > 0 {
			out.Labels = from.Context.Tags.Clone()
		}
		if from.Context.Message.IsSet() {
			out.Message = &model.Message{}
			if from.Context.Message.Age.IsSet() {
				val := from.Context.Message.Age.Milliseconds.Val
				out.Message.AgeMillis = &val
			}
			if from.Context.Message.Body.IsSet() {
				val := from.Context.Message.Body.Val
				out.Message.Body = &val
			}
			if from.Context.Message.Headers.IsSet() {
				out.Message.Headers = from.Context.Message.Headers.Val.Clone()
			}
			if from.Context.Message.Queue.IsSet() && from.Context.Message.Queue.Name.IsSet() {
				val := from.Context.Message.Queue.Name.Val
				out.Message.QueueName = &val
			}
		}
		if from.Context.Page.IsSet() {
			out.Page = &model.Page{}
			mapToPageModel(from.Context.Page, out.Page)
		}
		if from.Context.Request.IsSet() {
			out.HTTP = &model.Http{Request: &model.Req{}}
			mapToRequestModel(from.Context.Request, out.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				val := from.Context.Request.HTTPVersion.Val
				out.HTTP.Version = &val
			}
		}
		if from.Context.Request.URL.IsSet() {
			out.URL = &model.URL{}
			mapToRequestURLModel(from.Context.Request.URL, out.URL)
		}
		if from.Context.Response.IsSet() {
			if out.HTTP == nil {
				out.HTTP = &model.Http{}
			}
			out.HTTP.Response = &model.Resp{}
			mapToResponseModel(from.Context.Response, out.HTTP.Response)
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
	if from.SpanCount.Dropped.IsSet() {
		dropped := from.SpanCount.Dropped.Val
		out.SpanCount.Dropped = &dropped
	}
	if from.SpanCount.Started.IsSet() {
		started := from.SpanCount.Started.Val
		out.SpanCount.Started = &started
	}
	if from.Timestamp.Val.IsZero() {
		out.Timestamp = reqTime
	} else {
		out.Timestamp = from.Timestamp.Val
	}
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
