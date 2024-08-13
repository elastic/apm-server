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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/apm-data/input/elasticapm/internal/decoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/modeldecoderutil"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/netutil"
	"github.com/elastic/apm-data/input/elasticapm/internal/modeldecoder/nullable"
	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modelpb"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var compressionStrategyText = map[string]modelpb.CompressionStrategy{
	"exact_match": modelpb.CompressionStrategy_COMPRESSION_STRATEGY_EXACT_MATCH,
	"same_kind":   modelpb.CompressionStrategy_COMPRESSION_STRATEGY_SAME_KIND,
}

var metricTypeText = map[string]modelpb.MetricType{
	"gauge":     modelpb.MetricType_METRIC_TYPE_GAUGE,
	"counter":   modelpb.MetricType_METRIC_TYPE_COUNTER,
	"histogram": modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
	"summary":   modelpb.MetricType_METRIC_TYPE_SUMMARY,
}

var (
	// reForServiceTargetExpr regex will capture service target type and name
	// Service target type comprises of only lowercase alphabets
	// Service target name comprises of all word characters
	reForServiceTargetExpr = regexp.MustCompile(`^([a-z0-9]+)(?:/(\w+))?$`)
)

// DecodeMetadata decodes metadata from d, updating out.
//
// DecodeMetadata should be used when the the stream in the decoder does not contain the
// `metadata` key, but only the metadata data.
func DecodeMetadata(d decoder.Decoder, out *modelpb.APMEvent) error {
	return decodeMetadata(decodeIntoMetadata, d, out)
}

// DecodeNestedMetadata decodes metadata from d, updating out.
//
// DecodeNestedMetadata should be used when the stream in the decoder contains the `metadata` key
func DecodeNestedMetadata(d decoder.Decoder, out *modelpb.APMEvent) error {
	return decodeMetadata(decodeIntoMetadataRoot, d, out)
}

// DecodeNestedError decodes an error from d, appending it to batch.
//
// DecodeNestedError should be used when the stream in the decoder contains the `error` key
func DecodeNestedError(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := &errorRoot{}
	err := d.Decode(root)
	if err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base.CloneVT()
	mapToErrorModel(&root.Error, event)
	*batch = append(*batch, event)
	return err
}

// DecodeNestedMetricset decodes a metricset from d, appending it to batch.
//
// DecodeNestedMetricset should be used when the stream in the decoder contains the `metricset` key
func DecodeNestedMetricset(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := &metricsetRoot{}
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base.CloneVT()
	if mapToMetricsetModel(&root.Metricset, event) {
		*batch = append(*batch, event)
	}
	return err
}

// DecodeNestedSpan decodes a span from d, appending it to batch.
//
// DecodeNestedSpan should be used when the stream in the decoder contains the `span` key
func DecodeNestedSpan(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := &spanRoot{}
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base.CloneVT()
	mapToSpanModel(&root.Span, event)
	*batch = append(*batch, event)
	return err
}

// DecodeNestedTransaction decodes a transaction from d, appending it to batch.
//
// DecodeNestedTransaction should be used when the stream in the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := &transactionRoot{}
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base.CloneVT()
	mapToTransactionModel(&root.Transaction, event)
	*batch = append(*batch, event)
	return err
}

// DecodeNestedLog decodes a log event from d, appending it to batch.
//
// DecodeNestedLog should be used when the stream in the decoder contains the `log` key
func DecodeNestedLog(d decoder.Decoder, input *modeldecoder.Input, batch *modelpb.Batch) error {
	root := &logRoot{}
	var err error
	if err = d.Decode(root); err != nil && err != io.EOF {
		return modeldecoder.NewDecoderErrFromJSONIter(err)
	}
	// Flatten any nested source to set the values for the flat fields
	if err := root.processNestedSource(); err != nil {
		return err
	}
	if err := root.validate(); err != nil {
		return modeldecoder.NewValidationErr(err)
	}
	event := input.Base.CloneVT()
	mapToLogModel(&root.Log, event)
	*batch = append(*batch, event)
	return err
}

func decodeMetadata(decFn func(d decoder.Decoder, m *metadataRoot) error, d decoder.Decoder, out *modelpb.APMEvent) error {
	m := &metadataRoot{}
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

func mapToFAASModel(from faas, faas *modelpb.Faas) {
	if from.ID.IsSet() {
		faas.Id = from.ID.Val
	}
	if from.Coldstart.IsSet() {
		valCopy := from.Coldstart
		faas.ColdStart = &valCopy.Val
	}
	if from.Execution.IsSet() {
		faas.Execution = from.Execution.Val
	}
	if from.Trigger.Type.IsSet() {
		faas.TriggerType = from.Trigger.Type.Val
	}
	if from.Trigger.RequestID.IsSet() {
		faas.TriggerRequestId = from.Trigger.RequestID.Val
	}
	if from.Name.IsSet() {
		faas.Name = from.Name.Val
	}
	if from.Version.IsSet() {
		faas.Version = from.Version.Val
	}
}

func mapToDroppedSpansModel(from []transactionDroppedSpanStats, tx *modelpb.Transaction) {
	for _, f := range from {
		to := modelpb.DroppedSpanStats{}
		if f.DestinationServiceResource.IsSet() {
			to.DestinationServiceResource = f.DestinationServiceResource.Val
		}
		if f.Outcome.IsSet() {
			to.Outcome = f.Outcome.Val
		}
		if f.Duration.IsSet() {
			to.Duration = &modelpb.AggregatedDuration{}
			to.Duration.Count = uint64(f.Duration.Count.Val)
			sum := f.Duration.Sum
			if sum.IsSet() {
				to.Duration.Sum = uint64(time.Duration(sum.Us.Val) * time.Microsecond)
			}
		}
		if f.ServiceTargetType.IsSet() {
			to.ServiceTargetType = f.ServiceTargetType.Val
		}
		if f.ServiceTargetName.IsSet() {
			to.ServiceTargetName = f.ServiceTargetName.Val
		}

		tx.DroppedSpansStats = append(tx.DroppedSpansStats, &to)
	}
}

func mapToCloudModel(from contextCloud, cloud *modelpb.Cloud) {
	cloudOrigin := modelpb.CloudOrigin{}
	if from.Origin.Account.ID.IsSet() {
		cloudOrigin.AccountId = from.Origin.Account.ID.Val
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
	cloud.Origin = &cloudOrigin
}

func mapToClientModel(from contextRequest, source **modelpb.Source, client **modelpb.Client) {
	// http.Request.Headers and http.Request.Socket are only set for backend events.
	if (*source).GetIp() == nil {
		ip, port := netutil.SplitAddrPort(from.Socket.RemoteAddress.Val)
		if ip.IsValid() {
			if *source == nil {
				*source = &modelpb.Source{}
			}
			(*source).Ip, (*source).Port = modelpb.Addr2IP(ip), uint32(port)
		}
	}
	if (*client).GetIp() == nil {
		if (*source).GetIp() != nil {
			if *client == nil {
				*client = &modelpb.Client{}
			}
			(*client).Ip = (*source).Ip.CloneVT()
		}
		if addr, port := netutil.ClientAddrFromHeaders(from.Headers.Val); addr.IsValid() {
			if (*source).GetIp() != nil {
				(*source).Nat = &modelpb.NAT{}
				(*source).Nat.Ip = (*source).Ip.CloneVT()
			}
			if *client == nil {
				*client = &modelpb.Client{}
			}
			clientIp := modelpb.Addr2IP(addr)
			sourceIp := clientIp.CloneVT()

			(*client).Ip, (*client).Port = clientIp, uint32(port)
			if *source == nil {
				*source = &modelpb.Source{}
			}
			(*source).Ip, (*source).Port = sourceIp, uint32(port)
		}
	}
}

func mapToErrorModel(from *errorEvent, event *modelpb.APMEvent) {
	out := modelpb.Error{}
	event.Error = &out

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
		if from.Context.Request.URL.IsSet() {
			if event.Url == nil {
				event.Url = &modelpb.URL{}
			}
			mapToRequestURLModel(from.Context.Request.URL, event.Url)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() && !from.Context.Request.URL.IsSet() {
				event.Url = modelpb.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.Http == nil {
					event.Http = &modelpb.HTTP{}
				}
				if event.Http.Request == nil {
					event.Http.Request = &modelpb.HTTPRequest{}
				}
				if event.Http.Request.Referrer == "" {
					event.Http.Request.Referrer = from.Context.Page.Referer.Val
				}
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
	if from.TransactionID.IsSet() || from.Transaction.IsSet() {
		event.Transaction = &modelpb.Transaction{}
	}
	if from.TransactionID.IsSet() {
		event.Transaction.Id = from.TransactionID.Val
		event.Span = &modelpb.Span{}
		event.Span.Id = from.TransactionID.Val
	}
	if from.Transaction.IsSet() {
		if from.Transaction.Sampled.IsSet() {
			event.Transaction.Sampled = from.Transaction.Sampled.Val
		}
		if from.Transaction.Name.IsSet() {
			event.Transaction.Name = from.Transaction.Name.Val
		}
		if from.Transaction.Type.IsSet() {
			event.Transaction.Type = from.Transaction.Type.Val
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

func mapToMetadataModel(from *metadata, out *modelpb.APMEvent) {
	// Cloud
	if from.Cloud.IsSet() {
		if out.Cloud == nil {
			out.Cloud = &modelpb.Cloud{}
		}
		if from.Cloud.Account.ID.IsSet() {
			out.Cloud.AccountId = from.Cloud.Account.ID.Val
		}
		if from.Cloud.Account.Name.IsSet() {
			out.Cloud.AccountName = from.Cloud.Account.Name.Val
		}
		if from.Cloud.AvailabilityZone.IsSet() {
			out.Cloud.AvailabilityZone = from.Cloud.AvailabilityZone.Val
		}
		if from.Cloud.Instance.ID.IsSet() {
			out.Cloud.InstanceId = from.Cloud.Instance.ID.Val
		}
		if from.Cloud.Instance.Name.IsSet() {
			out.Cloud.InstanceName = from.Cloud.Instance.Name.Val
		}
		if from.Cloud.Machine.Type.IsSet() {
			out.Cloud.MachineType = from.Cloud.Machine.Type.Val
		}
		if from.Cloud.Project.ID.IsSet() {
			out.Cloud.ProjectId = from.Cloud.Project.ID.Val
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
	}

	// Labels
	if len(from.Labels) > 0 {
		modeldecoderutil.GlobalLabelsFrom(from.Labels, out)
	}

	// Process
	if len(from.Process.Argv) > 0 {
		if out.Process == nil {
			out.Process = &modelpb.Process{}
		}
		out.Process.Argv = append(out.Process.Argv[:0], from.Process.Argv...)
	}
	if from.Process.Pid.IsSet() {
		if out.Process == nil {
			out.Process = &modelpb.Process{}
		}
		out.Process.Pid = uint32(from.Process.Pid.Val)
	}
	if from.Process.Ppid.IsSet() {
		var pid = uint32(from.Process.Ppid.Val)
		if out.Process == nil {
			out.Process = &modelpb.Process{}
		}
		out.Process.Ppid = pid
	}
	if from.Process.Title.IsSet() {
		if out.Process == nil {
			out.Process = &modelpb.Process{}
		}
		out.Process.Title = from.Process.Title.Val
	}

	// Service
	if from.Service.Agent.IsSet() {
		if out.Agent == nil {
			out.Agent = &modelpb.Agent{}
		}
		if from.Service.Agent.ActivationMethod.IsSet() {
			out.Agent.ActivationMethod = from.Service.Agent.ActivationMethod.Val
		}
		if from.Service.Agent.EphemeralID.IsSet() {
			out.Agent.EphemeralId = from.Service.Agent.EphemeralID.Val
		}
		if from.Service.Agent.Name.IsSet() {
			out.Agent.Name = from.Service.Agent.Name.Val
		}
		if from.Service.Agent.Version.IsSet() {
			out.Agent.Version = from.Service.Agent.Version.Val
		}
	}
	if from.Service.IsSet() {
		if out.Service == nil {
			out.Service = &modelpb.Service{}
		}
		if from.Service.Environment.IsSet() {
			out.Service.Environment = from.Service.Environment.Val
		}
		if from.Service.Framework.IsSet() {
			if out.Service.Framework == nil {
				out.Service.Framework = &modelpb.Framework{}
			}
			if from.Service.Framework.Name.IsSet() {
				out.Service.Framework.Name = from.Service.Framework.Name.Val
			}
			if from.Service.Framework.Version.IsSet() {
				out.Service.Framework.Version = from.Service.Framework.Version.Val
			}
		}
		if from.Service.Language.IsSet() {
			if out.Service.Language == nil {
				out.Service.Language = &modelpb.Language{}
			}
			if from.Service.Language.Name.IsSet() {
				out.Service.Language.Name = from.Service.Language.Name.Val
			}
			if from.Service.Language.Version.IsSet() {
				out.Service.Language.Version = from.Service.Language.Version.Val
			}
		}
		if from.Service.Name.IsSet() {
			out.Service.Name = from.Service.Name.Val
		}
		if from.Service.Node.Name.IsSet() {
			if out.Service.Node == nil {
				out.Service.Node = &modelpb.ServiceNode{}
			}
			out.Service.Node.Name = from.Service.Node.Name.Val
		}
		if from.Service.Runtime.IsSet() {
			if out.Service.Runtime == nil {
				out.Service.Runtime = &modelpb.Runtime{}
			}
			if from.Service.Runtime.Name.IsSet() {
				out.Service.Runtime.Name = from.Service.Runtime.Name.Val
			}
			if from.Service.Runtime.Version.IsSet() {
				out.Service.Runtime.Version = from.Service.Runtime.Version.Val
			}
		}
		if from.Service.Version.IsSet() {
			out.Service.Version = from.Service.Version.Val
		}
	}

	// System
	if from.System.Architecture.IsSet() {
		if out.Host == nil {
			out.Host = &modelpb.Host{}
		}
		out.Host.Architecture = from.System.Architecture.Val
	}
	if from.System.ConfiguredHostname.IsSet() {
		if out.Host == nil {
			out.Host = &modelpb.Host{}
		}
		out.Host.Name = from.System.ConfiguredHostname.Val
	}
	if from.System.Container.ID.IsSet() {
		if out.Container == nil {
			out.Container = &modelpb.Container{}
		}
		out.Container.Id = from.System.Container.ID.Val
	}
	if from.System.DetectedHostname.IsSet() {
		if out.Host == nil {
			out.Host = &modelpb.Host{}
		}
		out.Host.Hostname = from.System.DetectedHostname.Val
	}
	if from.System.HostID.IsSet() {
		if out.Host == nil {
			out.Host = &modelpb.Host{}
		}
		out.Host.Id = from.System.HostID.Val
	}
	if !from.System.ConfiguredHostname.IsSet() && !from.System.DetectedHostname.IsSet() &&
		from.System.DeprecatedHostname.IsSet() {
		if out.Host == nil {
			out.Host = &modelpb.Host{}
		}
		out.Host.Hostname = from.System.DeprecatedHostname.Val
	}
	if from.System.Kubernetes.Namespace.IsSet() {
		if out.Kubernetes == nil {
			out.Kubernetes = &modelpb.Kubernetes{}
		}
		out.Kubernetes.Namespace = from.System.Kubernetes.Namespace.Val
	}
	if from.System.Kubernetes.Node.Name.IsSet() {
		if out.Kubernetes == nil {
			out.Kubernetes = &modelpb.Kubernetes{}
		}
		out.Kubernetes.NodeName = from.System.Kubernetes.Node.Name.Val
	}
	if from.System.Kubernetes.Pod.Name.IsSet() {
		if out.Kubernetes == nil {
			out.Kubernetes = &modelpb.Kubernetes{}
		}
		out.Kubernetes.PodName = from.System.Kubernetes.Pod.Name.Val
	}
	if from.System.Kubernetes.Pod.UID.IsSet() {
		if out.Kubernetes == nil {
			out.Kubernetes = &modelpb.Kubernetes{}
		}
		out.Kubernetes.PodUid = from.System.Kubernetes.Pod.UID.Val
	}
	if from.System.Platform.IsSet() {
		if out.Host == nil {
			out.Host = &modelpb.Host{}
		}
		if out.Host.Os == nil {
			out.Host.Os = &modelpb.OS{}
		}
		out.Host.Os.Platform = from.System.Platform.Val
	}

	// User
	if from.User.Domain.IsSet() {
		if out.User == nil {
			out.User = &modelpb.User{}
		}
		out.User.Domain = from.User.Domain.Val
	}
	if from.User.ID.IsSet() {
		if out.User == nil {
			out.User = &modelpb.User{}
		}
		out.User.Id = fmt.Sprint(from.User.ID.Val)
	}
	if from.User.Email.IsSet() {
		if out.User == nil {
			out.User = &modelpb.User{}
		}
		out.User.Email = from.User.Email.Val
	}
	if from.User.Name.IsSet() {
		if out.User == nil {
			out.User = &modelpb.User{}
		}
		out.User.Name = from.User.Name.Val
	}

	// Network
	if from.Network.Connection.Type.IsSet() {
		if out.Network == nil {
			out.Network = &modelpb.Network{}
		}
		if out.Network.Connection == nil {
			out.Network.Connection = &modelpb.NetworkConnection{}
		}
		out.Network.Connection.Type = from.Network.Connection.Type.Val
	}
}

func mapToMetricsetModel(from *metricset, event *modelpb.APMEvent) bool {
	event.Metricset = &modelpb.Metricset{}
	event.Metricset.Name = "app"

	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = modelpb.FromTime(from.Timestamp.Val)
	}

	if len(from.Samples) > 0 {
		event.Metricset.Samples = modeldecoderutil.ResliceAndPopulateNil(
			event.Metricset.Samples,
			len(from.Samples),
			modelpb.MetricsetSampleFromVTPool,
		)
		i := 0
		for name, sample := range from.Samples {
			ms := event.Metricset.Samples[i]
			if len(sample.Counts) != 0 || len(sample.Values) != 0 {
				ms.Histogram = &modelpb.Histogram{}
				if n := len(sample.Values); n > 0 {
					ms.Histogram.Values = modeldecoderutil.Reslice(ms.Histogram.Values, n)
					copy(ms.Histogram.Values, sample.Values)
				}
				if n := len(sample.Counts); n > 0 {
					ms.Histogram.Counts = modeldecoderutil.Reslice(ms.Histogram.Counts, n)
					copy(ms.Histogram.Counts, sample.Counts)
				}
			}
			ms.Type = metricTypeText[sample.Type.Val]
			ms.Name = name
			ms.Unit = sample.Unit.Val
			ms.Value = sample.Value.Val
			i++
		}
	}

	if len(from.Tags) > 0 {
		modeldecoderutil.MergeLabels(from.Tags, event)
	}

	if from.Span.IsSet() {
		event.Span = &modelpb.Span{}
		if from.Span.Subtype.IsSet() {
			event.Span.Subtype = from.Span.Subtype.Val
		}
		if from.Span.Type.IsSet() {
			event.Span.Type = from.Span.Type.Val
		}
	}

	ok := true
	if from.Transaction.IsSet() {
		event.Transaction = &modelpb.Transaction{}
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
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		event.Service.Name = from.Service.Name.Val
		event.Service.Version = from.Service.Version.Val
	}

	if from.FAAS.IsSet() {
		if event.Faas == nil {
			event.Faas = &modelpb.Faas{}
		}
		mapToFAASModel(from.FAAS, event.Faas)
	}

	return ok
}

func mapToRequestModel(from contextRequest, out *modelpb.HTTPRequest) {
	if from.Method.IsSet() {
		out.Method = from.Method.Val
	}
	if len(from.Env) > 0 {
		out.Env = modeldecoderutil.ToKv(from.Env, out.Env)
	}
	if from.Body.IsSet() {
		out.Body = modeldecoderutil.ToValue(modeldecoderutil.NormalizeHTTPRequestBody(from.Body.Val))
	}
	if len(from.Cookies) > 0 {
		out.Cookies = modeldecoderutil.ToKv(from.Cookies, out.Cookies)
	}
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Headers.Val, out.Headers)
	}
}

func mapToRequestURLModel(from contextRequestURL, out *modelpb.URL) {
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
			out.Port = uint32(port)
		}
	}
}

func mapToResponseModel(from contextResponse, out *modelpb.HTTPResponse) {
	if from.Finished.IsSet() {
		val := from.Finished.Val
		out.Finished = &val
	}
	if from.Headers.IsSet() {
		out.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Headers.Val, out.Headers)
	}
	if from.HeadersSent.IsSet() {
		val := from.HeadersSent.Val
		out.HeadersSent = &val
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

func mapToServiceModel(from contextService, outPtr **modelpb.Service) {
	var out *modelpb.Service
	if *outPtr == nil {
		*outPtr = &modelpb.Service{}
	}
	out = *outPtr
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
	if from.Node.Name.IsSet() {
		if out.Node == nil {
			out.Node = &modelpb.ServiceNode{}
		}
		out.Node.Name = from.Node.Name.Val
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
	if from.Origin.IsSet() {
		outOrigin := modelpb.ServiceOrigin{}
		if from.Origin.ID.IsSet() {
			outOrigin.Id = from.Origin.ID.Val
		}
		if from.Origin.Name.IsSet() {
			outOrigin.Name = from.Origin.Name.Val
		}
		if from.Origin.Version.IsSet() {
			outOrigin.Version = from.Origin.Version.Val
		}
		out.Origin = &outOrigin
	}
	if from.Target.IsSet() {
		outTarget := modelpb.ServiceTarget{}
		if from.Target.Name.IsSet() {
			outTarget.Name = from.Target.Name.Val
		}
		if from.Target.Type.IsSet() {
			outTarget.Type = from.Target.Type.Val
		}
		out.Target = &outTarget
	}
}

func mapToAgentModel(from contextServiceAgent, out **modelpb.Agent) {
	if *out == nil {
		*out = &modelpb.Agent{}
	}
	if from.Name.IsSet() {
		(*out).Name = from.Name.Val
	}
	if from.Version.IsSet() {
		(*out).Version = from.Version.Val
	}
	if from.EphemeralID.IsSet() {
		(*out).EphemeralId = from.EphemeralID.Val
	}
}

func mapToSpanModel(from *span, event *modelpb.APMEvent) {
	out := modelpb.Span{}
	event.Span = &out

	// map span specific data
	if !from.Action.IsSet() && !from.Subtype.IsSet() {
		sep := "."
		before, after, ok := strings.Cut(from.Type.Val, sep)
		out.Type = before
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
	if from.Composite.IsSet() {
		composite := modelpb.Composite{}
		if from.Composite.Count.IsSet() {
			composite.Count = uint32(from.Composite.Count.Val)
		}
		if from.Composite.Sum.IsSet() {
			composite.Sum = from.Composite.Sum.Val
		}
		if strategy, ok := compressionStrategyText[from.Composite.CompressionStrategy.Val]; ok {
			composite.CompressionStrategy = strategy
		}
		out.Composite = &composite
	}
	if len(from.ChildIDs) > 0 {
		event.ChildIds = modeldecoderutil.Reslice(event.ChildIds, len(from.ChildIDs))
		copy(event.ChildIds, from.ChildIDs)
	}
	if from.Context.Database.IsSet() {
		db := modelpb.DB{}
		if from.Context.Database.Instance.IsSet() {
			db.Instance = from.Context.Database.Instance.Val
		}
		if from.Context.Database.Link.IsSet() {
			db.Link = from.Context.Database.Link.Val
		}
		if from.Context.Database.RowsAffected.IsSet() {
			val := uint32(from.Context.Database.RowsAffected.Val)
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
		out.Db = &db
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
		if from.Context.HTTP.Method.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			if event.Http.Request == nil {
				event.Http.Request = &modelpb.HTTPRequest{}
			}
			event.Http.Request.Method = from.Context.HTTP.Method.Val
		}
		if from.Context.HTTP.Request.ID.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			if event.Http.Request == nil {
				event.Http.Request = &modelpb.HTTPRequest{}
			}
			event.Http.Request.Id = from.Context.HTTP.Request.ID.Val
		}
		if from.Context.HTTP.Response.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			response := modelpb.HTTPResponse{}
			if from.Context.HTTP.Response.DecodedBodySize.IsSet() {
				val := uint64(from.Context.HTTP.Response.DecodedBodySize.Val)
				response.DecodedBodySize = &val
			}
			if from.Context.HTTP.Response.EncodedBodySize.IsSet() {
				val := uint64(from.Context.HTTP.Response.EncodedBodySize.Val)
				response.EncodedBodySize = &val
			}
			if from.Context.HTTP.Response.Headers.IsSet() {
				response.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Context.HTTP.Response.Headers.Val, response.Headers)
			}
			if from.Context.HTTP.Response.StatusCode.IsSet() {
				response.StatusCode = uint32(from.Context.HTTP.Response.StatusCode.Val)
			}
			if from.Context.HTTP.Response.TransferSize.IsSet() {
				val := uint64(from.Context.HTTP.Response.TransferSize.Val)
				response.TransferSize = &val
			}
			event.Http.Response = &response
		}
		if from.Context.HTTP.StatusCode.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			if event.Http.Response == nil {
				event.Http.Response = &modelpb.HTTPResponse{}
			}
			event.Http.Response.StatusCode = uint32(from.Context.HTTP.StatusCode.Val)
		}
		if from.Context.HTTP.URL.IsSet() {
			if event.Url == nil {
				event.Url = &modelpb.URL{}
			}
			event.Url.Original = from.Context.HTTP.URL.Val
		}
	}
	if from.Context.Message.IsSet() {
		message := modelpb.Message{}
		if from.Context.Message.Body.IsSet() {
			message.Body = from.Context.Message.Body.Val
		}
		if from.Context.Message.Headers.IsSet() {
			message.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Context.Message.Headers.Val, message.Headers)
		}
		if from.Context.Message.Age.Milliseconds.IsSet() {
			val := uint64(from.Context.Message.Age.Milliseconds.Val)
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
	if !from.Context.Service.Target.Type.IsSet() && from.Context.Destination.Service.Resource.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		outTarget := targetFromDestinationResource(from.Context.Destination.Service.Resource.Val)
		event.Service.Target = outTarget
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
	if from.ParentID.IsSet() {
		event.ParentId = from.ParentID.Val
	}
	if from.SampleRate.IsSet() {
		if from.SampleRate.Val > 0 {
			out.RepresentativeCount = 1 / from.SampleRate.Val
		}
	} else {
		// NOTE: we don't know the sample rate, so we need to make an assumption.
		//
		// Agents never send spans for non-sampled transactions, so assuming a
		// representative count of 1 (i.e. sampling rate of 100%) may be invalid; but
		// this is more useful than producing no metrics at all (i.e. assuming 0).
		out.RepresentativeCount = 1
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
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = modelpb.FromTime(from.Timestamp.Val)
	} else if from.Start.IsSet() {
		// event.Timestamp should have been initialized to the time the
		// payload was received; offset that by "start" milliseconds for
		// RUM.
		event.Timestamp += uint64(float64(time.Millisecond) * from.Start.Val)
	}
	if from.TraceID.IsSet() {
		event.Trace = &modelpb.Trace{}
		event.Trace.Id = from.TraceID.Val
	}
	if from.TransactionID.IsSet() {
		event.Transaction = &modelpb.Transaction{}
		event.Transaction.Id = from.TransactionID.Val
	}
	if from.OTel.IsSet() {
		mapOTelAttributesSpan(from.OTel, event)
	}
	if len(from.Links) > 0 {
		out.Links = modeldecoderutil.ResliceAndPopulateNil(
			out.Links,
			len(from.Links),
			modelpb.SpanLinkFromVTPool,
		)
		mapSpanLinks(from.Links, out.Links)
	}
	if out.Type == "" {
		out.Type = "unknown"
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
		if eventFrame.LibraryFrame.IsSet() {
			val := eventFrame.LibraryFrame.Val
			fr.LibraryFrame = val
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
		if len(eventFrame.Vars) > 0 {
			fr.Vars = modeldecoderutil.ToKv(eventFrame.Vars, fr.Vars)
		}
	}
}

func mapToTransactionModel(from *transaction, event *modelpb.APMEvent) {
	out := modelpb.Transaction{}
	event.Transaction = &out

	// overwrite metadata with event specific information
	mapToServiceModel(from.Context.Service, &event.Service)
	mapToAgentModel(from.Context.Service.Agent, &event.Agent)
	overwriteUserInMetadataModel(from.Context.User, event)
	mapToUserAgentModel(from.Context.Request.Headers, &event.UserAgent)
	mapToClientModel(from.Context.Request, &event.Source, &event.Client)
	if from.FAAS.IsSet() {
		if event.Faas == nil {
			event.Faas = &modelpb.Faas{}
		}
		mapToFAASModel(from.FAAS, event.Faas)
	}
	if from.Context.Cloud.IsSet() {
		if event.Cloud == nil {
			event.Cloud = &modelpb.Cloud{}
		}
		mapToCloudModel(from.Context.Cloud, event.Cloud)
	}
	mapToDroppedSpansModel(from.DroppedSpanStats, event.Transaction)

	// map transaction specific data

	if from.Context.IsSet() {
		if len(from.Context.Custom) > 0 {
			out.Custom = modeldecoderutil.ToKv(from.Context.Custom, out.Custom)
		}
		if len(from.Context.Tags) > 0 {
			modeldecoderutil.MergeLabels(from.Context.Tags, event)
		}
		if from.Context.Message.IsSet() {
			out.Message = &modelpb.Message{}
			if from.Context.Message.Age.IsSet() {
				val := uint64(from.Context.Message.Age.Milliseconds.Val)
				out.Message.AgeMillis = &val
			}
			if from.Context.Message.Body.IsSet() {
				out.Message.Body = from.Context.Message.Body.Val
			}
			if from.Context.Message.Headers.IsSet() {
				out.Message.Headers = modeldecoderutil.HTTPHeadersToModelpb(from.Context.Message.Headers.Val, out.Message.Headers)
			}
			if from.Context.Message.Queue.IsSet() && from.Context.Message.Queue.Name.IsSet() {
				out.Message.QueueName = from.Context.Message.Queue.Name.Val
			}
			if from.Context.Message.RoutingKey.IsSet() {
				out.Message.RoutingKey = from.Context.Message.RoutingKey.Val
			}
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
		if from.Context.Request.URL.IsSet() {
			if event.Url == nil {
				event.Url = &modelpb.URL{}
			}
			mapToRequestURLModel(from.Context.Request.URL, event.Url)
		}
		if from.Context.Response.IsSet() {
			if event.Http == nil {
				event.Http = &modelpb.HTTP{}
			}
			event.Http.Response = &modelpb.HTTPResponse{}
			mapToResponseModel(from.Context.Response, event.Http.Response)
		}
		if from.Context.Page.IsSet() {
			if from.Context.Page.URL.IsSet() && !from.Context.Request.URL.IsSet() {
				event.Url = modelpb.ParseURL(from.Context.Page.URL.Val, "", "")
			}
			if from.Context.Page.Referer.IsSet() {
				if event.Http == nil {
					event.Http = &modelpb.HTTP{}
				}
				if event.Http.Request == nil {
					event.Http.Request = &modelpb.HTTPRequest{}
				}
				if event.Http.Request.Referrer == "" {
					event.Http.Request.Referrer = from.Context.Page.Referer.Val
				}
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
		event.Span = &modelpb.Span{}
		event.Span.Id = from.ID.Val
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
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = modelpb.FromTime(from.Timestamp.Val)
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

	if from.OTel.IsSet() {
		if event.Span == nil {
			event.Span = &modelpb.Span{}
		}
		mapOTelAttributesTransaction(from.OTel, event)
	}

	if len(from.Links) > 0 {
		if event.Span == nil {
			event.Span = &modelpb.Span{}
		}
		event.Span.Links = modeldecoderutil.ResliceAndPopulateNil(
			event.Span.Links,
			len(from.Links),
			modelpb.SpanLinkFromVTPool,
		)
		mapSpanLinks(from.Links, event.Span.Links)
	}
	if out.Type == "" {
		out.Type = "unknown"
	}
}

func mapToLogModel(from *log, event *modelpb.APMEvent) {
	if from.FAAS.IsSet() {
		if event.Faas == nil {
			event.Faas = &modelpb.Faas{}
		}
		mapToFAASModel(from.FAAS, event.Faas)
	}
	if !from.Timestamp.Val.IsZero() {
		event.Timestamp = modelpb.FromTime(from.Timestamp.Val)
	}
	if from.TraceID.IsSet() {
		event.Trace = &modelpb.Trace{}
		event.Trace.Id = from.TraceID.Val
	}
	if from.TransactionID.IsSet() {
		event.Transaction = &modelpb.Transaction{}
		event.Transaction.Id = from.TransactionID.Val
		event.Span = &modelpb.Span{}
		event.Span.Id = from.TransactionID.Val
	}
	if from.SpanID.IsSet() {
		event.Span = &modelpb.Span{}
		event.Span.Id = from.SpanID.Val
	}
	if from.Message.IsSet() {
		event.Message = from.Message.Val
	}
	if from.Level.IsSet() {
		if event.Log == nil {
			event.Log = &modelpb.Log{}
		}
		event.Log.Level = from.Level.Val
	}
	if from.Logger.IsSet() {
		if event.Log == nil {
			event.Log = &modelpb.Log{}
		}
		event.Log.Logger = from.Logger.Val
	}
	if from.OriginFunction.IsSet() {
		if event.Log == nil {
			event.Log = &modelpb.Log{}
		}
		if event.Log.Origin == nil {
			event.Log.Origin = &modelpb.LogOrigin{}
		}
		event.Log.Origin.FunctionName = from.OriginFunction.Val
	}
	if from.OriginFileLine.IsSet() {
		if event.Log == nil {
			event.Log = &modelpb.Log{}
		}
		if event.Log.Origin == nil {
			event.Log.Origin = &modelpb.LogOrigin{}
		}
		if event.Log.Origin.File == nil {
			event.Log.Origin.File = &modelpb.LogOriginFile{}
		}
		event.Log.Origin.File.Line = uint32(from.OriginFileLine.Val)
	}
	if from.OriginFileName.IsSet() {
		if event.Log == nil {
			event.Log = &modelpb.Log{}
		}
		if event.Log.Origin == nil {
			event.Log.Origin = &modelpb.LogOrigin{}
		}
		if event.Log.Origin.File == nil {
			event.Log.Origin.File = &modelpb.LogOriginFile{}
		}
		event.Log.Origin.File.Name = from.OriginFileName.Val
	}
	if from.ErrorType.IsSet() ||
		from.ErrorMessage.IsSet() ||
		from.ErrorStacktrace.IsSet() {
		event.Error = &modelpb.Error{}
		event.Error.Message = from.ErrorMessage.Val
		event.Error.Type = from.ErrorType.Val
		event.Error.StackTrace = from.ErrorStacktrace.Val
	}
	if from.ServiceName.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		event.Service.Name = from.ServiceName.Val
	}
	if from.ServiceVersion.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		event.Service.Version = from.ServiceVersion.Val
	}
	if from.ServiceEnvironment.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		event.Service.Environment = from.ServiceEnvironment.Val
	}
	if from.ServiceNodeName.IsSet() {
		if event.Service == nil {
			event.Service = &modelpb.Service{}
		}
		if event.Service.Node == nil {
			event.Service.Node = &modelpb.ServiceNode{}
		}
		event.Service.Node.Name = from.ServiceNodeName.Val
	}
	if from.ProcessThreadName.IsSet() {
		if event.Process == nil {
			event.Process = &modelpb.Process{}
		}
		if event.Process.Thread == nil {
			event.Process.Thread = &modelpb.ProcessThread{}
		}
		event.Process.Thread.Name = from.ProcessThreadName.Val
	}
	if from.EventDataset.IsSet() {
		if event.Event == nil {
			event.Event = &modelpb.Event{}
		}
		event.Event.Dataset = from.EventDataset.Val
	}
	if len(from.Labels) > 0 {
		modeldecoderutil.MergeLabels(from.Labels, event)
	}
	if event.Error == nil {
		if event.Event == nil {
			event.Event = &modelpb.Event{}
		}
		event.Event.Kind = "event"
	}
}

func mapOTelAttributesTransaction(from otel, out *modelpb.APMEvent) {
	scope := pcommon.NewInstrumentationScope()
	m := otelAttributeMap(&from)
	if from.SpanKind.IsSet() {
		out.Span.Kind = from.SpanKind.Val
	}
	if out.Labels == nil {
		out.Labels = make(modelpb.Labels)
	}
	if out.NumericLabels == nil {
		out.NumericLabels = make(modelpb.NumericLabels)
	}
	// TODO: Does this work? Is there a way we can infer the status code,
	// potentially in the actual attributes map?
	spanStatus := ptrace.NewStatus()
	spanStatus.SetCode(ptrace.StatusCodeUnset)
	otlp.TranslateTransaction(m, spanStatus, scope, out)

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

const (
	spanKindInternal = "INTERNAL"
	spanKindServer   = "SERVER"
	spanKindClient   = "CLIENT"
	spanKindProducer = "PRODUCER"
	spanKindConsumer = "CONSUMER"
)

func mapOTelAttributesSpan(from otel, out *modelpb.APMEvent) {
	m := otelAttributeMap(&from)
	if out.Labels == nil {
		out.Labels = make(modelpb.Labels)
	}
	if out.NumericLabels == nil {
		out.NumericLabels = make(modelpb.NumericLabels)
	}
	var spanKind ptrace.SpanKind
	if from.SpanKind.IsSet() {
		switch from.SpanKind.Val {
		case spanKindInternal:
			spanKind = ptrace.SpanKindInternal
		case spanKindServer:
			spanKind = ptrace.SpanKindServer
		case spanKindClient:
			spanKind = ptrace.SpanKindClient
		case spanKindProducer:
			spanKind = ptrace.SpanKindProducer
		case spanKindConsumer:
			spanKind = ptrace.SpanKindConsumer
		default:
			spanKind = ptrace.SpanKindUnspecified
		}
		out.Span.Kind = from.SpanKind.Val
	}
	otlp.TranslateSpan(spanKind, m, out)

	if spanKind == ptrace.SpanKindUnspecified {
		switch out.Span.Type {
		case "db", "external", "storage":
			out.Span.Kind = "CLIENT"
		default:
			out.Span.Kind = "INTERNAL"
		}
	}
}

func mapToUserAgentModel(from nullable.HTTPHeader, out **modelpb.UserAgent) {
	// overwrite userAgent information if available
	if from.IsSet() {
		if h := from.Val.Values(textproto.CanonicalMIMEHeaderKey("User-Agent")); len(h) > 0 {
			if *out == nil {
				*out = &modelpb.UserAgent{}
			}
			(*out).Original = strings.Join(h, ", ")
		}
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

func otelAttributeMap(o *otel) pcommon.Map {
	m := pcommon.NewMap()
	for k, v := range o.Attributes {
		if attr, ok := otelAttributeValue(k, v); ok {
			attr.CopyTo(m.PutEmpty(k))
		}
	}
	return m
}

func otelAttributeValue(k string, v interface{}) (pcommon.Value, bool) {
	// According to the spec, these are the allowed primitive types
	// Additionally, homogeneous arrays (single type) of primitive types are allowed
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/common/common.md#attributes
	switch v := v.(type) {
	case string:
		return pcommon.NewValueStr(v), true
	case bool:
		return pcommon.NewValueBool(v), true
	case json.Number:
		// Semantic conventions have specified types, and we rely on this
		// in processor/otel when mapping to our data model. For example,
		// `http.status_code` is expected to be an int.
		if !isOTelDoubleAttribute(k) {
			if v, err := v.Int64(); err == nil {
				return pcommon.NewValueInt(v), true
			}
		}
		if v, err := v.Float64(); err == nil {
			return pcommon.NewValueDouble(v), true
		}
	case []interface{}:
		array := pcommon.NewValueSlice()
		array.Slice().EnsureCapacity(len(v))
		for i := range v {
			if elem, ok := otelAttributeValue(k, v[i]); ok {
				elem.CopyTo(array.Slice().AppendEmpty())
			}
		}
		return array, true
	}
	return pcommon.Value{}, false
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

func mapSpanLinks(from []spanLink, out []*modelpb.SpanLink) {
	for i, link := range from {
		out[i].SpanId = link.SpanID.Val
		out[i].TraceId = link.TraceID.Val
	}
}

func targetFromDestinationResource(res string) *modelpb.ServiceTarget {
	target := modelpb.ServiceTarget{}
	submatch := reForServiceTargetExpr.FindStringSubmatch(res)
	switch len(submatch) {
	case 3:
		target.Type = submatch[1]
		target.Name = submatch[2]
	default:
		target.Name = res
	}
	return &target
}
