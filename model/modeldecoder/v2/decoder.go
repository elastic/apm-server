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

func fetchMetadataRoot() *metadataRoot {
	return metadataRootPool.Get().(*metadataRoot)
}

func releaseMetadataRoot(root *metadataRoot) {
	root.Reset()
	metadataRootPool.Put(root)
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

// DecodeNestedTransaction uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Transaction instance
//
// DecodeNestedTransaction should be used when the stream in the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, out *model.Transaction) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	var err error
	if err = d.Decode(&root); err != nil && err != io.EOF {
		return fmt.Errorf("decode error %w", err)
	}
	if err := root.validate(); err != nil {
		return fmt.Errorf("validation error %w", err)
	}
	mapToTransactionModel(&root.Transaction, &input.Metadata, input.RequestTime, input.Config.Experimental, out)
	return err
}

func decodeMetadata(decFn func(d decoder.Decoder, m *metadataRoot) error, d decoder.Decoder, out *model.Metadata) error {
	m := fetchMetadataRoot()
	defer releaseMetadataRoot(m)
	var err error
	if err = decFn(d, m); err != nil && err != io.EOF {
		return fmt.Errorf("decode error %w", err)
	}
	if err := m.validate(); err != nil {
		return fmt.Errorf("validation error %w", err)
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

func mapToMetadataModel(from *metadata, out *model.Metadata) {
	// Cloud
	if from == nil {
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
		out.Process.Argv = from.Process.Argv
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
		from.System.HostnameDeprecated.IsSet() {
		out.System.DetectedHostname = from.System.HostnameDeprecated.Val
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

func mapToTransactionModel(from *transaction, metadata *model.Metadata, reqTime time.Time, experimental bool, out *model.Transaction) {
	// set metadata information
	out.Metadata = *metadata
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
			custom := model.Custom(from.Context.Custom.Clone())
			out.Custom = &custom
		}
		// metadata labels and context labels are merged only in the output model
		if len(from.Context.Tags) > 0 {
			labels := model.Labels(from.Context.Tags.Clone())
			out.Labels = &labels
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

	// TODO(simitt): set accordingly, once this is fixed:
	// https://github.com/elastic/apm-server/issues/4188
	// if t.SampleRate.IsSet() {}

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
	if experimental {
		out.Experimental = from.Experimental.Val
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
