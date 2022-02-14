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
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/internal/netutil"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/modeldecoderutil"
	"github.com/elastic/apm-server/model/modeldecoder/nullable"
	otel_processor "github.com/elastic/apm-server/processor/otel"

	"go.opentelemetry.io/collector/model/pdata"
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

// DecodeMetadata decodes metadata from d, updating out.
//
// DecodeMetadata should be used when the the stream in the decoder does not contain the
// `metadata` key, but only the metadata data.
func DecodeMetadata(d decoder.Decoder, out *model.APMEvent) error {
	return decodeMetadata(decodeIntoMetadata, d, out)
}

// DecodeNestedMetadata decodes metadata from d, updating out.
//
// DecodeNestedMetadata should be used when the stream in the decoder contains the `metadata` key
func DecodeNestedMetadata(d decoder.Decoder, out *model.APMEvent) error {
	return decodeMetadata(decodeIntoMetadataRoot, d, out)
}

// DecodeNestedError decodes an error from d, appending it to batch.
//
// DecodeNestedError should be used when the stream in the decoder contains the `error` key
func DecodeNestedError(d decoder.Decoder, input *modeldecoder.Input, batch *model.Batch) error {
	root := fetchErrorRoot()
	defer releaseErrorRoot(root)
	err := d.Decode(root)
	if err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base
	mapToErrorModel(&root.Error, &event)
	*batch = append(*batch, event)
	return err
}

// DecodeNestedMetricset decodes a metricset from d, appending it to batch.
//
// DecodeNestedMetricset should be used when the stream in the decoder contains the `metricset` key
func DecodeNestedMetricset(d decoder.Decoder, input *modeldecoder.Input, batch *model.Batch) error {
	root := fetchMetricsetRoot()
	defer releaseMetricsetRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base
	if mapToMetricsetModel(&root.Metricset, &event) {
		*batch = append(*batch, event)
	}
	return err
}

// DecodeNestedSpan decodes a span from d, appending it to batch.
//
// DecodeNestedSpan should be used when the stream in the decoder contains the `span` key
func DecodeNestedSpan(d decoder.Decoder, input *modeldecoder.Input, batch *model.Batch) error {
	root := fetchSpanRoot()
	defer releaseSpanRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base
	mapToSpanModel(&root.Span, &event)
	*batch = append(*batch, event)
	return err
}

// DecodeNestedTransaction decodes a transaction from d, appending it to batch.
//
// DecodeNestedTransaction should be used when the stream in the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, batch *model.Batch) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base
	mapToTransactionModel(&root.Transaction, &event)
	*batch = append(*batch, event)
	return err
}

func decodeMetadata(decFn func(d decoder.Decoder, m *metadataRoot) error, d decoder.Decoder, out *model.APMEvent) error {
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

func mapToFAASModel(from faas, faas *model.FAAS) {
	if from.IsSet() {
		if from.ID.IsSet() {
			faas.ID = from.ID.Val
		}
		if from.Coldstart.IsSet() {
			faas.Coldstart = &from.Coldstart.Val
		}
		if from.Execution.IsSet() {
			faas.Execution = from.Execution.Val
		}
		if from.Trigger.Type.IsSet() {
			faas.TriggerType = from.Trigger.Type.Val
		}
		if from.Trigger.RequestID.IsSet() {
			faas.TriggerRequestID = from.Trigger.RequestID.Val
		}
	}
}

func mapToDroppedSpansModel(from []transactionDroppedSpanStats, tx *model.Transaction) {
	for _, f := range from {
		if f.IsSet() {
			var to model.DroppedSpanStats
			if f.DestinationServiceResource.IsSet() {
				to.DestinationServiceResource = f.DestinationServiceResource.Val
			}
			if f.Outcome.IsSet() {
				to.Outcome = f.Outcome.Val
			}

			if f.Duration.IsSet() {
				to.Duration.Count = f.Duration.Count.Val
				sum := f.Duration.Sum
				if sum.IsSet() {
					to.Duration.Sum = time.Duration(sum.Us.Val) * time.Microsecond
				}
			}

			tx.DroppedSpansStats = append(tx.DroppedSpansStats, to)
		}
	}
}

func mapToCloudModel(from contextCloud, cloud *model.Cloud) {
	if from.IsSet() {
		cloudOrigin := &model.CloudOrigin{}
		if from.Origin.Account.ID.IsSet() {
			cloudOrigin.AccountID = from.Origin.Account.ID.Val
		}
		if from.Origin.Provider.IsSet() {
			cloudOrigin.Provider = from.Origin.Provider.Val
		}
		if from.Origin.Region.IsSet() {
			cloudOrigin.Region = from.Origin.Region.Val
		}
		if from.Origin.Service.Name.IsSet() {
			cloudOrigin.ServiceName = from.Origin.Service.Name.Val
		}
		cloud.Origin = cloudOrigin
	}
}

func mapToClientModel(from contextRequest, source *model.Source, client *model.Client) {
	// http.Request.Headers and http.Request.Socket are only set for backend events.
	if source.IP == nil {
		ip, port := netutil.ParseIPPort(
			netutil.MaybeSplitHostPort(from.Socket.RemoteAddress.Val),
		)
		source.IP, source.Port = ip, int(port)
	}
	if client.IP == nil {
		client.IP = source.IP
		if ip, port := netutil.ClientAddrFromHeaders(from.Headers.Val); ip != nil {
			client.IP, client.Port = ip, int(port)
		}
	}
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
	mapToClientModel(from.Context.Request, &event.Source, &event.Client)

	// map errorEvent specific data

	if from.Context.IsSet() {
		if len(from.Context.Tags) > 0 {
			modeldecoderutil.MergeLabels(from.Context.Tags, event)
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
		if from.Context.Request.URL.IsSet() {
			mapToRequestURLModel(from.Context.Request.URL, &event.URL)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() && !from.Context.Request.URL.IsSet() {
				event.URL = model.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.HTTP.Request == nil {
					event.HTTP.Request = &model.HTTPRequest{}
				}
				if event.HTTP.Request.Referrer == "" {
					event.HTTP.Request.Referrer = from.Context.Page.Referer.Val
				}
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
		log := model.ErrorLog{}
		if from.Log.Level.IsSet() {
			log.Level = from.Log.Level.Val
		}
		if from.Log.LoggerName.IsSet() {
			log.LoggerName = from.Log.LoggerName.Val
		}
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
		if from.Transaction.Name.IsSet() {
			event.Transaction.Name = from.Transaction.Name.Val
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

func mapToMetadataModel(from *metadata, out *model.APMEvent) {
	// Cloud
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
	if from.Cloud.Service.Name.IsSet() {
		out.Cloud.ServiceName = from.Cloud.Service.Name.Val
	}

	// Labels
	if len(from.Labels) > 0 {
		modeldecoderutil.LabelsFrom(from.Labels, out)
	}

	// Process
	if len(from.Process.Argv) > 0 {
		out.Process.Argv = append(out.Process.Argv[:0], from.Process.Argv...)
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
		out.Agent.EphemeralID = from.Service.Agent.EphemeralID.Val
	}
	if from.Service.Agent.Name.IsSet() {
		out.Agent.Name = from.Service.Agent.Name.Val
	}
	if from.Service.Agent.Version.IsSet() {
		out.Agent.Version = from.Service.Agent.Version.Val
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
		out.Host.Architecture = from.System.Architecture.Val
	}
	if from.System.ConfiguredHostname.IsSet() {
		out.Host.Name = from.System.ConfiguredHostname.Val
	}
	if from.System.Container.ID.IsSet() {
		out.Container.ID = from.System.Container.ID.Val
	}
	if from.System.DetectedHostname.IsSet() {
		out.Host.Hostname = from.System.DetectedHostname.Val
	}
	if !from.System.ConfiguredHostname.IsSet() && !from.System.DetectedHostname.IsSet() &&
		from.System.DeprecatedHostname.IsSet() {
		out.Host.Hostname = from.System.DeprecatedHostname.Val
	}
	if from.System.Kubernetes.Namespace.IsSet() {
		out.Kubernetes.Namespace = from.System.Kubernetes.Namespace.Val
	}
	if from.System.Kubernetes.Node.Name.IsSet() {
		out.Kubernetes.NodeName = from.System.Kubernetes.Node.Name.Val
	}
	if from.System.Kubernetes.Pod.Name.IsSet() {
		out.Kubernetes.PodName = from.System.Kubernetes.Pod.Name.Val
	}
	if from.System.Kubernetes.Pod.UID.IsSet() {
		out.Kubernetes.PodUID = from.System.Kubernetes.Pod.UID.Val
	}
	if from.System.Platform.IsSet() {
		out.Host.OS.Platform = from.System.Platform.Val
	}

	// User
	if from.User.Domain.IsSet() {
		out.User.Domain = fmt.Sprint(from.User.Domain.Val)
	}
	if from.User.ID.IsSet() {
		out.User.ID = fmt.Sprint(from.User.ID.Val)
	}
	if from.User.Email.IsSet() {
		out.User.Email = from.User.Email.Val
	}
	if from.User.Name.IsSet() {
		out.User.Name = from.User.Name.Val
	}

	// Network
	if from.Network.Connection.Type.IsSet() {
		out.Network.Connection.Type = from.Network.Connection.Type.Val
	}
}

func mapToMetricsetModel(from *metricset, event *model.APMEvent) bool {
	event.Metricset = &model.Metricset{}
	event.Processor = model.MetricsetProcessor

	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = from.Timestamp.Val
	}

	if len(from.Samples) > 0 {
		samples := make(map[string]model.MetricsetSample, len(from.Samples))
		for name, sample := range from.Samples {
			var counts []int64
			var values []float64
			if n := len(sample.Values); n > 0 {
				values = make([]float64, n)
				copy(values, sample.Values)
			}
			if n := len(sample.Counts); n > 0 {
				counts = make([]int64, n)
				copy(counts, sample.Counts)
			}
			samples[name] = model.MetricsetSample{
				Type:  model.MetricType(sample.Type.Val),
				Unit:  sample.Unit.Val,
				Value: sample.Value.Val,
				Histogram: model.Histogram{
					Values: values,
					Counts: counts,
				},
			}
		}
		event.Metricset.Samples = samples
	}

	if len(from.Tags) > 0 {
		modeldecoderutil.MergeLabels(from.Tags, event)
	}

	if from.Span.IsSet() {
		event.Span = &model.Span{}
		if from.Span.Subtype.IsSet() {
			event.Span.Subtype = from.Span.Subtype.Val
		}
		if from.Span.Type.IsSet() {
			event.Span.Type = from.Span.Type.Val
		}
	}

	ok := true
	if from.Transaction.IsSet() {
		event.Transaction = &model.Transaction{}
		if from.Transaction.Name.IsSet() {
			event.Transaction.Name = from.Transaction.Name.Val
		}
		if from.Transaction.Type.IsSet() {
			event.Transaction.Type = from.Transaction.Type.Val
		}
		// Transaction fields specified: this is an APM-internal metricset.
		// If there are no known metric samples, we return false so the
		// metricset is not added to the batch.
		ok = modeldecoderutil.SetInternalMetrics(event)
	}

	if from.Service.Name.IsSet() {
		event.Service.Name = from.Service.Name.Val
		event.Service.Version = from.Service.Version.Val
	}

	return ok
}

func mapToRequestModel(from contextRequest, out *model.HTTPRequest) {
	if from.Method.IsSet() {
		out.Method = from.Method.Val
	}
	if len(from.Env) > 0 {
		out.Env = from.Env.Clone()
	}
	if from.Body.IsSet() {
		out.Body = modeldecoderutil.NormalizeHTTPRequestBody(from.Body.Val)
	}
	if len(from.Cookies) > 0 {
		out.Cookies = from.Cookies.Clone()
	}
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToMap(from.Headers.Val.Clone())
	}
}

func mapToRequestURLModel(from contextRequestURL, out *model.URL) {
	if from.Raw.IsSet() {
		out.Original = from.Raw.Val
	}
	if from.Full.IsSet() {
		out.Full = from.Full.Val
	}
	if from.Hostname.IsSet() {
		out.Domain = from.Hostname.Val
	}
	if from.Path.IsSet() {
		out.Path = from.Path.Val
	}
	if from.Search.IsSet() {
		out.Query = from.Search.Val
	}
	if from.Hash.IsSet() {
		out.Fragment = from.Hash.Val
	}
	if from.Protocol.IsSet() {
		out.Scheme = strings.TrimSuffix(from.Protocol.Val, ":")
	}
	if from.Port.IsSet() {
		// should never result in an error, type is checked when decoding
		port, err := strconv.Atoi(fmt.Sprint(from.Port.Val))
		if err == nil {
			out.Port = port
		}
	}
}

func mapToResponseModel(from contextResponse, out *model.HTTPResponse) {
	if from.Finished.IsSet() {
		val := from.Finished.Val
		out.Finished = &val
	}
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToMap(from.Headers.Val.Clone())
	}
	if from.HeadersSent.IsSet() {
		val := from.HeadersSent.Val
		out.HeadersSent = &val
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
	if from.Origin.IsSet() {
		outOrigin := &model.ServiceOrigin{}
		if from.Origin.ID.IsSet() {
			outOrigin.ID = from.Origin.ID.Val
		}
		if from.Origin.Name.IsSet() {
			outOrigin.Name = from.Origin.Name.Val
		}
		if from.Origin.Version.IsSet() {
			outOrigin.Version = from.Origin.Version.Val
		}
		out.Origin = outOrigin
	}
}

func mapToAgentModel(from contextServiceAgent, out *model.Agent) {
	if from.Name.IsSet() {
		out.Name = from.Name.Val
	}
	if from.Version.IsSet() {
		out.Version = from.Version.Val
	}
	if from.EphemeralID.IsSet() {
		out.EphemeralID = from.EphemeralID.Val
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
	if from.Composite.IsSet() {
		composite := model.Composite{}
		if from.Composite.Count.IsSet() {
			composite.Count = from.Composite.Count.Val
		}
		if from.Composite.Sum.IsSet() {
			composite.Sum = from.Composite.Sum.Val
		}
		if from.Composite.CompressionStrategy.IsSet() {
			composite.CompressionStrategy = from.Composite.CompressionStrategy.Val
		}
		out.Composite = &composite
	}
	if len(from.ChildIDs) > 0 {
		event.Child.ID = make([]string, len(from.ChildIDs))
		copy(event.Child.ID, from.ChildIDs)
	}
	if from.Context.Database.IsSet() {
		db := model.DB{}
		if from.Context.Database.Instance.IsSet() {
			db.Instance = from.Context.Database.Instance.Val
		}
		if from.Context.Database.Link.IsSet() {
			db.Link = from.Context.Database.Link.Val
		}
		if from.Context.Database.RowsAffected.IsSet() {
			val := from.Context.Database.RowsAffected.Val
			db.RowsAffected = &val
		}
		if from.Context.Database.Statement.IsSet() {
			db.Statement = from.Context.Database.Statement.Val
		}
		if from.Context.Database.Type.IsSet() {
			db.Type = from.Context.Database.Type.Val
		}
		if from.Context.Database.User.IsSet() {
			db.UserName = from.Context.Database.User.Val
		}
		out.DB = &db
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
		if from.Context.HTTP.Method.IsSet() {
			event.HTTP.Request = &model.HTTPRequest{}
			event.HTTP.Request.Method = from.Context.HTTP.Method.Val
		}
		if from.Context.HTTP.Response.IsSet() {
			response := model.HTTPResponse{}
			if from.Context.HTTP.Response.DecodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.DecodedBodySize.Val
				response.DecodedBodySize = &val
			}
			if from.Context.HTTP.Response.EncodedBodySize.IsSet() {
				val := from.Context.HTTP.Response.EncodedBodySize.Val
				response.EncodedBodySize = &val
			}
			if from.Context.HTTP.Response.Headers.IsSet() {
				response.Headers = modeldecoderutil.HTTPHeadersToMap(from.Context.HTTP.Response.Headers.Val.Clone())
			}
			if from.Context.HTTP.Response.StatusCode.IsSet() {
				response.StatusCode = from.Context.HTTP.Response.StatusCode.Val
			}
			if from.Context.HTTP.Response.TransferSize.IsSet() {
				val := from.Context.HTTP.Response.TransferSize.Val
				response.TransferSize = &val
			}
			event.HTTP.Response = &response
		}
		if from.Context.HTTP.StatusCode.IsSet() {
			if event.HTTP.Response == nil {
				event.HTTP.Response = &model.HTTPResponse{}
			}
			event.HTTP.Response.StatusCode = from.Context.HTTP.StatusCode.Val
		}
		if from.Context.HTTP.URL.IsSet() {
			event.URL.Original = from.Context.HTTP.URL.Val
		}
	}
	if from.Context.Message.IsSet() {
		message := model.Message{}
		if from.Context.Message.Body.IsSet() {
			message.Body = from.Context.Message.Body.Val
		}
		if from.Context.Message.Headers.IsSet() {
			message.Headers = from.Context.Message.Headers.Val.Clone()
		}
		if from.Context.Message.Age.Milliseconds.IsSet() {
			val := from.Context.Message.Age.Milliseconds.Val
			message.AgeMillis = &val
		}
		if from.Context.Message.Queue.Name.IsSet() {
			message.QueueName = from.Context.Message.Queue.Name.Val
		}
		if from.Context.Message.RoutingKey.IsSet() {
			message.RoutingKey = from.Context.Message.RoutingKey.Val
		}
		out.Message = &message
	}
	if from.Context.Service.IsSet() {
		mapToServiceModel(from.Context.Service, &event.Service)
		mapToAgentModel(from.Context.Service.Agent, &event.Agent)
	}
	if len(from.Context.Tags) > 0 {
		modeldecoderutil.MergeLabels(from.Context.Tags, event)
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
	if from.ParentID.IsSet() {
		event.Parent.ID = from.ParentID.Val
	}
	if from.SampleRate.IsSet() && from.SampleRate.Val > 0 {
		out.RepresentativeCount = 1 / from.SampleRate.Val
	}
	if len(from.Stacktrace) > 0 {
		out.Stacktrace = make(model.Stacktrace, len(from.Stacktrace))
		mapToStracktraceModel(from.Stacktrace, out.Stacktrace)
	}
	if from.Sync.IsSet() {
		val := from.Sync.Val
		out.Sync = &val
	}
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = from.Timestamp.Val
	} else if from.Start.IsSet() {
		// event.Timestamp is initialized to the time the payload was
		// received by apm-server; offset that by "start" milliseconds
		// for RUM.
		event.Timestamp = event.Timestamp.Add(
			time.Duration(float64(time.Millisecond) * from.Start.Val),
		)
	}
	if from.TraceID.IsSet() {
		event.Trace.ID = from.TraceID.Val
	}
	if from.TransactionID.IsSet() {
		event.Transaction = &model.Transaction{ID: from.TransactionID.Val}
	}
	if from.OTel.IsSet() {
		mapOTelAttributesSpan(from.OTel, event)
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
		if eventFrame.LibraryFrame.IsSet() {
			val := eventFrame.LibraryFrame.Val
			fr.LibraryFrame = val
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
		if len(eventFrame.Vars) > 0 {
			fr.Vars = eventFrame.Vars.Clone()
		}
		out[idx] = &fr
	}
}

func mapToTransactionModel(from *transaction, event *model.APMEvent) {
	out := &model.Transaction{}
	event.Processor = model.TransactionProcessor
	event.Transaction = out

	// overwrite metadata with event specific information
	mapToServiceModel(from.Context.Service, &event.Service)
	mapToAgentModel(from.Context.Service.Agent, &event.Agent)
	overwriteUserInMetadataModel(from.Context.User, event)
	mapToUserAgentModel(from.Context.Request.Headers, &event.UserAgent)
	mapToClientModel(from.Context.Request, &event.Source, &event.Client)
	mapToFAASModel(from.FAAS, &event.FAAS)
	mapToCloudModel(from.Context.Cloud, &event.Cloud)
	mapToDroppedSpansModel(from.DroppedSpanStats, event.Transaction)

	// map transaction specific data

	if from.Context.IsSet() {
		if len(from.Context.Custom) > 0 {
			out.Custom = modeldecoderutil.NormalizeLabelValues(from.Context.Custom.Clone())
		}
		if len(from.Context.Tags) > 0 {
			modeldecoderutil.MergeLabels(from.Context.Tags, event)
		}
		if from.Context.Message.IsSet() {
			out.Message = &model.Message{}
			if from.Context.Message.Age.IsSet() {
				val := from.Context.Message.Age.Milliseconds.Val
				out.Message.AgeMillis = &val
			}
			if from.Context.Message.Body.IsSet() {
				out.Message.Body = from.Context.Message.Body.Val
			}
			if from.Context.Message.Headers.IsSet() {
				out.Message.Headers = from.Context.Message.Headers.Val.Clone()
			}
			if from.Context.Message.Queue.IsSet() && from.Context.Message.Queue.Name.IsSet() {
				out.Message.QueueName = from.Context.Message.Queue.Name.Val
			}
			if from.Context.Message.RoutingKey.IsSet() {
				out.Message.RoutingKey = from.Context.Message.RoutingKey.Val
			}
		}
		if from.Context.Request.IsSet() {
			event.HTTP.Request = &model.HTTPRequest{}
			mapToRequestModel(from.Context.Request, event.HTTP.Request)
			if from.Context.Request.HTTPVersion.IsSet() {
				event.HTTP.Version = from.Context.Request.HTTPVersion.Val
			}
		}
		if from.Context.Request.URL.IsSet() {
			mapToRequestURLModel(from.Context.Request.URL, &event.URL)
		}
		if from.Context.Response.IsSet() {
			event.HTTP.Response = &model.HTTPResponse{}
			mapToResponseModel(from.Context.Response, event.HTTP.Response)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() && !from.Context.Request.URL.IsSet() {
				event.URL = model.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.HTTP.Request == nil {
					event.HTTP.Request = &model.HTTPRequest{}
				}
				if event.HTTP.Request.Referrer == "" {
					event.HTTP.Request.Referrer = from.Context.Page.Referer.Val
				}
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
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = from.Timestamp.Val
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

	if from.OTel.IsSet() {
		if event.Span == nil {
			event.Span = &model.Span{}
		}
		mapOTelAttributesTransaction(from.OTel, event)
	}
}

func mapOTelAttributesTransaction(from otel, out *model.APMEvent) {
	library := pdata.NewInstrumentationLibrary()
	m := otelAttributeMap(&from)
	if from.SpanKind.IsSet() {
		out.Span.Kind = from.SpanKind.Val
	}
	if out.Labels == nil {
		out.Labels = make(model.Labels)
	}
	if out.NumericLabels == nil {
		out.NumericLabels = make(model.NumericLabels)
	}
	// TODO: Does this work? Is there a way we can infer the status code,
	// potentially in the actual attributes map?
	spanStatus := pdata.NewSpanStatus()
	spanStatus.SetCode(pdata.StatusCodeUnset)
	otel_processor.TranslateTransaction(m, spanStatus, library, out)

	if out.Span.Kind == "" {
		switch out.Transaction.Type {
		case "messaging":
			out.Span.Kind = "CONSUMER"
		case "request":
			out.Span.Kind = "SERVER"
		default:
			out.Span.Kind = "INTERNAL"
		}
	}
}

const spanKindStringPrefix = "SPAN_KIND_"

func mapOTelAttributesSpan(from otel, out *model.APMEvent) {
	m := otelAttributeMap(&from)
	if out.Labels == nil {
		out.Labels = make(model.Labels)
	}
	if out.NumericLabels == nil {
		out.NumericLabels = make(model.NumericLabels)
	}
	var spanKind pdata.SpanKind
	if from.SpanKind.IsSet() {
		switch from.SpanKind.Val {
		case pdata.SpanKindInternal.String()[len(spanKindStringPrefix):]:
			spanKind = pdata.SpanKindInternal
		case pdata.SpanKindServer.String()[len(spanKindStringPrefix):]:
			spanKind = pdata.SpanKindServer
		case pdata.SpanKindClient.String()[len(spanKindStringPrefix):]:
			spanKind = pdata.SpanKindClient
		case pdata.SpanKindProducer.String()[len(spanKindStringPrefix):]:
			spanKind = pdata.SpanKindProducer
		case pdata.SpanKindConsumer.String()[len(spanKindStringPrefix):]:
			spanKind = pdata.SpanKindConsumer
		default:
			spanKind = pdata.SpanKindUnspecified
		}
		out.Span.Kind = from.SpanKind.Val
	}
	otel_processor.TranslateSpan(spanKind, m, out)

	if spanKind == pdata.SpanKindUnspecified {
		switch out.Span.Type {
		case "db", "external", "storage":
			out.Span.Kind = "CLIENT"
		default:
			out.Span.Kind = "INTERNAL"
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

func otelAttributeMap(o *otel) pdata.AttributeMap {
	m := pdata.NewAttributeMap()
	for k, v := range o.Attributes {
		if attr, ok := otelAttributeValue(k, v); ok {
			m.Insert(k, attr)
		}
	}
	return m
}

func otelAttributeValue(k string, v interface{}) (pdata.AttributeValue, bool) {
	// According to the spec, these are the allowed primitive types
	// Additionally, homogeneous arrays (single type) of primitive types are allowed
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/common/common.md#attributes
	switch v := v.(type) {
	case string:
		return pdata.NewAttributeValueString(v), true
	case bool:
		return pdata.NewAttributeValueBool(v), true
	case json.Number:
		// Semantic conventions have specified types, and we rely on this
		// in processor/otel when mapping to our data model. For example,
		// `http.status_code` is expected to be an int.
		if !isOTelDoubleAttribute(k) {
			if v, err := v.Int64(); err == nil {
				return pdata.NewAttributeValueInt(v), true
			}
		}
		if v, err := v.Float64(); err == nil {
			return pdata.NewAttributeValueDouble(v), true
		}
	case []interface{}:
		array := pdata.NewAttributeValueArray()
		array.SliceVal().EnsureCapacity(len(v))
		for i := range v {
			if elem, ok := otelAttributeValue(k, v[i]); ok {
				elem.CopyTo(array.SliceVal().AppendEmpty())
			}
		}
		return array, true
	}
	return pdata.AttributeValue{}, false
}

// isOTelDoubleAttribute indicates whether k is an OpenTelemetry semantic convention attribute
// known to have type "double". As this list grows over time, we should consider generating
// the mapping with OpenTelemetry's semconvgen build tool.
//
// For the canonical semantic convention definitions, see
// https://github.com/open-telemetry/opentelemetry-specification/tree/main/semantic_conventions/trace
func isOTelDoubleAttribute(k string) bool {
	switch k {
	case "aws.dynamodb.provisioned_read_capacity":
		return true
	case "aws.dynamodb.provisioned_write_capacity":
		return true
	}
	return false
}
