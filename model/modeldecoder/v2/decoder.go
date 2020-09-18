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
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
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

// DecodeNestedTransaction uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Transaction instance
//
// DecodeNestedTransaction should be used when the decoder contains the `transaction` key
func DecodeNestedTransaction(d decoder.Decoder, input *modeldecoder.Input, out *model.Transaction) error {
	root := fetchTransactionRoot()
	defer releaseTransactionRoot(root)
	if err := d.Decode(&root); err != nil {
		return fmt.Errorf("decode error %w", err)
	}
	if err := root.validate(); err != nil {
		return fmt.Errorf("validation error %w", err)
	}
	mapToTransactionModel(&root.Transaction, &input.Metadata, input.RequestTime, input.Config.Experimental, out)
	return nil
}

// DecodeMetadata uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Metadata instance
//
// DecodeMetadata should be used when the underlying byte stream does not contain the
// `metadata` key, but only the metadata data.
func DecodeMetadata(d decoder.Decoder, out *model.Metadata) error {
	return decodeMetadata(decodeIntoMetadata, d, out)
}

// DecodeNestedMetadata uses the given decoder to create the input model,
// then runs the defined validations on the input model
// and finally maps the values fom the input model to the given *model.Metadata instance
//
// DecodeNestedMetadata should be used when the underlying byte stream does start with the `metadata` key
func DecodeNestedMetadata(d decoder.Decoder, out *model.Metadata) error {
	return decodeMetadata(decodeIntoMetadataRoot, d, out)
}

func decodeMetadata(decFn func(d decoder.Decoder, m *metadataRoot) error, d decoder.Decoder, out *model.Metadata) error {
	m := fetchMetadataRoot()
	defer releaseMetadataRoot(m)
	if err := decFn(d, m); err != nil {
		return fmt.Errorf("decode error %w", err)
	}
	if err := m.validate(); err != nil {
		return fmt.Errorf("validation error %w", err)
	}
	mapToMetadataModel(&m.Metadata, out)
	return nil
}

func decodeIntoMetadata(d decoder.Decoder, m *metadataRoot) error {
	return d.Decode(&m.Metadata)
}

func decodeIntoMetadataRoot(d decoder.Decoder, m *metadataRoot) error {
	return d.Decode(m)
}

func mapToMetadataModel(m *metadata, out *model.Metadata) {
	// Cloud
	if m == nil {
		return
	}
	if m.Cloud.Account.ID.IsSet() {
		out.Cloud.AccountID = m.Cloud.Account.ID.Val
	}
	if m.Cloud.Account.Name.IsSet() {
		out.Cloud.AccountName = m.Cloud.Account.Name.Val
	}
	if m.Cloud.AvailabilityZone.IsSet() {
		out.Cloud.AvailabilityZone = m.Cloud.AvailabilityZone.Val
	}
	if m.Cloud.Instance.ID.IsSet() {
		out.Cloud.InstanceID = m.Cloud.Instance.ID.Val
	}
	if m.Cloud.Instance.Name.IsSet() {
		out.Cloud.InstanceName = m.Cloud.Instance.Name.Val
	}
	if m.Cloud.Machine.Type.IsSet() {
		out.Cloud.MachineType = m.Cloud.Machine.Type.Val
	}
	if m.Cloud.Project.ID.IsSet() {
		out.Cloud.ProjectID = m.Cloud.Project.ID.Val
	}
	if m.Cloud.Project.Name.IsSet() {
		out.Cloud.ProjectName = m.Cloud.Project.Name.Val
	}
	if m.Cloud.Provider.IsSet() {
		out.Cloud.Provider = m.Cloud.Provider.Val
	}
	if m.Cloud.Region.IsSet() {
		out.Cloud.Region = m.Cloud.Region.Val
	}

	// Labels
	if len(m.Labels) > 0 {
		out.Labels = common.MapStr{}
		out.Labels.Update(m.Labels)
	}

	// Process
	if len(m.Process.Argv) > 0 {
		out.Process.Argv = m.Process.Argv
	}
	if m.Process.Pid.IsSet() {
		out.Process.Pid = m.Process.Pid.Val
	}
	if m.Process.Ppid.IsSet() {
		var pid = m.Process.Ppid.Val
		out.Process.Ppid = &pid
	}
	if m.Process.Title.IsSet() {
		out.Process.Title = m.Process.Title.Val
	}

	// Service
	if m.Service.Agent.EphemeralID.IsSet() {
		out.Service.Agent.EphemeralID = m.Service.Agent.EphemeralID.Val
	}
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
	if m.Service.Node.Name.IsSet() {
		out.Service.Node.Name = m.Service.Node.Name.Val
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

	// System
	if m.System.Architecture.IsSet() {
		out.System.Architecture = m.System.Architecture.Val
	}
	if m.System.ConfiguredHostname.IsSet() {
		out.System.ConfiguredHostname = m.System.ConfiguredHostname.Val
	}
	if m.System.Container.ID.IsSet() {
		out.System.Container.ID = m.System.Container.ID.Val
	}
	if m.System.DetectedHostname.IsSet() {
		out.System.DetectedHostname = m.System.DetectedHostname.Val
	}
	if !m.System.ConfiguredHostname.IsSet() && !m.System.DetectedHostname.IsSet() &&
		m.System.HostnameDeprecated.IsSet() {
		out.System.DetectedHostname = m.System.HostnameDeprecated.Val
	}
	if m.System.Kubernetes.Namespace.IsSet() {
		out.System.Kubernetes.Namespace = m.System.Kubernetes.Namespace.Val
	}
	if m.System.Kubernetes.Node.Name.IsSet() {
		out.System.Kubernetes.NodeName = m.System.Kubernetes.Node.Name.Val
	}
	if m.System.Kubernetes.Pod.Name.IsSet() {
		out.System.Kubernetes.PodName = m.System.Kubernetes.Pod.Name.Val
	}
	if m.System.Kubernetes.Pod.UID.IsSet() {
		out.System.Kubernetes.PodUID = m.System.Kubernetes.Pod.UID.Val
	}
	if m.System.Platform.IsSet() {
		out.System.Platform = m.System.Platform.Val
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

func mapToTransactionModel(t *transaction, metadata *model.Metadata, reqTime time.Time, experimental bool, out *model.Transaction) {
	if t == nil {
		return
	}

	// prefill with metadata information, then overwrite with event specific metadata
	out.Metadata = *metadata

	// only set metadata Labels
	out.Metadata.Labels = metadata.Labels.Clone()

	// overwrite Service values if set
	if t.Context.Service.Agent.EphemeralID.IsSet() {
		out.Metadata.Service.Agent.EphemeralID = t.Context.Service.Agent.EphemeralID.Val
	}
	if t.Context.Service.Agent.Name.IsSet() {
		out.Metadata.Service.Agent.Name = t.Context.Service.Agent.Name.Val
	}
	if t.Context.Service.Agent.Version.IsSet() {
		out.Metadata.Service.Agent.Version = t.Context.Service.Agent.Version.Val
	}
	if t.Context.Service.Environment.IsSet() {
		out.Metadata.Service.Environment = t.Context.Service.Environment.Val
	}
	if t.Context.Service.Framework.Name.IsSet() {
		out.Metadata.Service.Framework.Name = t.Context.Service.Framework.Name.Val
	}
	if t.Context.Service.Framework.Version.IsSet() {
		out.Metadata.Service.Framework.Version = t.Context.Service.Framework.Version.Val
	}
	if t.Context.Service.Language.Name.IsSet() {
		out.Metadata.Service.Language.Name = t.Context.Service.Language.Name.Val
	}
	if t.Context.Service.Language.Version.IsSet() {
		out.Metadata.Service.Language.Version = t.Context.Service.Language.Version.Val
	}
	if t.Context.Service.Name.IsSet() {
		out.Metadata.Service.Name = t.Context.Service.Name.Val
	}
	if t.Context.Service.Node.Name.IsSet() {
		out.Metadata.Service.Node.Name = t.Context.Service.Node.Name.Val
	}
	if t.Context.Service.Runtime.Name.IsSet() {
		out.Metadata.Service.Runtime.Name = t.Context.Service.Runtime.Name.Val
	}
	if t.Context.Service.Runtime.Version.IsSet() {
		out.Metadata.Service.Runtime.Version = t.Context.Service.Runtime.Version.Val
	}
	if t.Context.Service.Version.IsSet() {
		out.Metadata.Service.Version = t.Context.Service.Version.Val
	}

	// overwrite User specific values if set
	// either populate all User fields or none to avoid mixing
	// different user data
	if t.Context.User.ID.IsSet() || t.Context.User.Email.IsSet() || t.Context.User.Name.IsSet() {
		out.Metadata.User = model.User{}
		if t.Context.User.ID.IsSet() {
			out.Metadata.User.ID = fmt.Sprint(t.Context.User.ID.Val)
		}
		if t.Context.User.Email.IsSet() {
			out.Metadata.User.Email = t.Context.User.Email.Val
		}
		if t.Context.User.Name.IsSet() {
			out.Metadata.User.Name = t.Context.User.Name.Val
		}
	}

	if t.Context.Request.Headers.IsSet() {
		if h := t.Context.Request.Headers.Val.Values(textproto.CanonicalMIMEHeaderKey("User-Agent")); len(h) > 0 {
			out.Metadata.UserAgent.Original = strings.Join(h, ", ")
		}
	}

	// only set client information if not already set in metadata
	// this is aligned with current logic
	if out.Metadata.Client.IP == nil {
		// http.Request.Headers and http.Request.Socket information is
		// only set for backend events; try to first extract an IP address
		// from the headers, if not possible use IP address from socket remote_address
		if ip := utility.ExtractIPFromHeader(t.Context.Request.Headers.Val); ip != nil {
			out.Metadata.Client.IP = ip
		} else if ip := utility.ParseIP(t.Context.Request.Socket.RemoteAddress.Val); ip != nil {
			out.Metadata.Client.IP = ip
		}
	}

	// fill with event specific information

	// metadata labels and context labels are not merged at decoder level
	// but in the output model
	if len(t.Context.Tags) > 0 {
		labels := model.Labels(t.Context.Tags.Clone())
		out.Labels = &labels
	}
	if t.Duration.IsSet() {
		out.Duration = t.Duration.Val
	}
	if t.ID.IsSet() {
		out.ID = t.ID.Val
	}
	if t.Marks.IsSet() {
		out.Marks = make(model.TransactionMarks, len(t.Marks.Events))
		for event, val := range t.Marks.Events {
			if len(val.Measurements) > 0 {
				out.Marks[event] = model.TransactionMark(val.Measurements)
			}
		}
	}
	if t.Name.IsSet() {
		out.Name = t.Name.Val
	}
	if t.Outcome.IsSet() {
		out.Outcome = t.Outcome.Val
	} else {
		if t.Context.Response.StatusCode.IsSet() {
			statusCode := t.Context.Response.StatusCode.Val
			if statusCode >= http.StatusInternalServerError {
				out.Outcome = "failure"
			} else {
				out.Outcome = "success"
			}
		} else {
			out.Outcome = "unknown"
		}
	}
	if t.ParentID.IsSet() {
		out.ParentID = t.ParentID.Val
	}
	if t.Result.IsSet() {
		out.Result = t.Result.Val
	}

	out.Sampled = true
	if t.Sampled.IsSet() {
		out.Sampled = t.Sampled.Val
	}

	// TODO(simitt): set accordingly, once this is fixed:
	// https://github.com/elastic/apm-server/issues/4188
	// if t.SampleRate.IsSet() {}

	if t.SpanCount.Dropped.IsSet() {
		dropped := t.SpanCount.Dropped.Val
		out.SpanCount.Dropped = &dropped
	}
	if t.SpanCount.Started.IsSet() {
		started := t.SpanCount.Started.Val
		out.SpanCount.Started = &started
	}
	if t.Timestamp.Val.IsZero() {
		out.Timestamp = reqTime
	} else {
		out.Timestamp = t.Timestamp.Val
	}
	if t.TraceID.IsSet() {
		out.TraceID = t.TraceID.Val
	}
	if t.Type.IsSet() {
		out.Type = t.Type.Val
	}
	if t.UserExperience.IsSet() {
		out.UserExperience = &model.UserExperience{}
		if t.UserExperience.CumulativeLayoutShift.IsSet() {
			out.UserExperience.CumulativeLayoutShift = t.UserExperience.CumulativeLayoutShift.Val
		}
		if t.UserExperience.FirstInputDelay.IsSet() {
			out.UserExperience.FirstInputDelay = t.UserExperience.FirstInputDelay.Val

		}
		if t.UserExperience.TotalBlockingTime.IsSet() {
			out.UserExperience.TotalBlockingTime = t.UserExperience.TotalBlockingTime.Val
		}
	}
	if t.Context.IsSet() {
		if t.Context.Page.IsSet() {
			out.Page = &model.Page{}
			if t.Context.Page.URL.IsSet() {
				out.Page.URL = model.ParseURL(t.Context.Page.URL.Val, "")
			}
			if t.Context.Page.Referer.IsSet() {
				referer := t.Context.Page.Referer.Val
				out.Page.Referer = &referer
			}
		}

		if t.Context.Request.IsSet() {
			var request model.Req
			if t.Context.Request.Method.IsSet() {
				request.Method = t.Context.Request.Method.Val
			}
			if t.Context.Request.Env.IsSet() {
				request.Env = t.Context.Request.Env.Val
			}
			if t.Context.Request.Socket.IsSet() {
				request.Socket = &model.Socket{}
				if t.Context.Request.Socket.Encrypted.IsSet() {
					val := t.Context.Request.Socket.Encrypted.Val
					request.Socket.Encrypted = &val
				}
				if t.Context.Request.Socket.RemoteAddress.IsSet() {
					val := t.Context.Request.Socket.RemoteAddress.Val
					request.Socket.RemoteAddress = &val
				}
			}
			if t.Context.Request.Body.IsSet() {
				request.Body = t.Context.Request.Body.Val
			}
			if t.Context.Request.Cookies.IsSet() {
				request.Cookies = t.Context.Request.Cookies.Val
			}
			if t.Context.Request.Headers.IsSet() {
				request.Headers = t.Context.Request.Headers.Val.Clone()
			}
			out.HTTP = &model.Http{Request: &request}
			if t.Context.Request.HTTPVersion.IsSet() {
				val := t.Context.Request.HTTPVersion.Val
				out.HTTP.Version = &val
			}
		}
		if t.Context.Response.IsSet() {
			if out.HTTP == nil {
				out.HTTP = &model.Http{}
			}
			var response model.Resp
			if t.Context.Response.Finished.IsSet() {
				val := t.Context.Response.Finished.Val
				response.Finished = &val
			}
			if t.Context.Response.Headers.IsSet() {
				response.Headers = t.Context.Response.Headers.Val.Clone()
			}
			if t.Context.Response.HeadersSent.IsSet() {
				val := t.Context.Response.HeadersSent.Val
				response.HeadersSent = &val
			}
			if t.Context.Response.StatusCode.IsSet() {
				val := t.Context.Response.StatusCode.Val
				response.StatusCode = &val
			}
			if t.Context.Response.TransferSize.IsSet() {
				val := t.Context.Response.TransferSize.Val
				response.TransferSize = &val
			}
			if t.Context.Response.EncodedBodySize.IsSet() {
				val := t.Context.Response.EncodedBodySize.Val
				response.EncodedBodySize = &val
			}
			if t.Context.Response.DecodedBodySize.IsSet() {
				val := t.Context.Response.DecodedBodySize.Val
				response.DecodedBodySize = &val
			}
			out.HTTP.Response = &response
		}
		if t.Context.Request.URL.IsSet() {
			out.URL = &model.URL{}
			if t.Context.Request.URL.Raw.IsSet() {
				val := t.Context.Request.URL.Raw.Val
				out.URL.Original = &val
			}
			if t.Context.Request.URL.Full.IsSet() {
				val := t.Context.Request.URL.Full.Val
				out.URL.Full = &val
			}
			if t.Context.Request.URL.Hostname.IsSet() {
				val := t.Context.Request.URL.Hostname.Val
				out.URL.Domain = &val
			}
			if t.Context.Request.URL.Path.IsSet() {
				val := t.Context.Request.URL.Path.Val
				out.URL.Path = &val
			}
			if t.Context.Request.URL.Search.IsSet() {
				val := t.Context.Request.URL.Search.Val
				out.URL.Query = &val
			}
			if t.Context.Request.URL.Hash.IsSet() {
				val := t.Context.Request.URL.Hash.Val
				out.URL.Fragment = &val
			}
			if t.Context.Request.URL.Protocol.IsSet() {
				trimmed := strings.TrimSuffix(t.Context.Request.URL.Protocol.Val, ":")
				out.URL.Scheme = &trimmed
			}
			if t.Context.Request.URL.Port.IsSet() {
				port, err := strconv.Atoi(fmt.Sprint(t.Context.Request.URL.Port.Val))
				if err == nil {
					out.URL.Port = &port
				}
			}
		}
		if len(t.Context.Custom) > 0 {
			custom := model.Custom(t.Context.Custom.Clone())
			out.Custom = &custom
		}
		if t.Context.Message.IsSet() {
			out.Message = &model.Message{}
			if t.Context.Message.Age.IsSet() {
				val := t.Context.Message.Age.Milliseconds.Val
				out.Message.AgeMillis = &val
			}
			if t.Context.Message.Body.IsSet() {
				val := t.Context.Message.Body.Val
				out.Message.Body = &val
			}
			if t.Context.Message.Headers.IsSet() {
				out.Message.Headers = t.Context.Message.Headers.Val.Clone()
			}
			if t.Context.Message.Queue.IsSet() && t.Context.Message.Queue.Name.IsSet() {
				val := t.Context.Message.Queue.Name.Val
				out.Message.QueueName = &val
			}
		}
	}
	if experimental {
		out.Experimental = t.Experimental.Val
	}
}
