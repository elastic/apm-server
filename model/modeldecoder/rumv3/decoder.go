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
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
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

// DecodeNestedMetadata uses the given decoder to create the input models,
// then runs the defined validations on the input models
// and finally maps the values fom the input model to the given *model.Metadata instance
func DecodeNestedMetadata(d decoder.Decoder, out *model.Metadata) error {
	m := fetchMetadataRoot()
	defer releaseMetadataRoot(m)
	if err := d.Decode(&m); err != nil {
		return fmt.Errorf("decode error %w", err)
	}
	if err := m.validate(); err != nil {
		return fmt.Errorf("validation error %w", err)
	}
	mapToMetadataModel(&m.Metadata, out)
	return nil
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

func mapToTransactionModel(t *transaction, metadata *model.Metadata, reqTime time.Time, experimental bool, out *model.Transaction) {
	if t == nil {
		return
	}

	// prefill with metadata information, then overwrite with event specific metadata
	out.Metadata = *metadata

	// only set metadata Labels
	out.Metadata.Labels = metadata.Labels.Clone()

	// overwrite Service values if set
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

	sampled := true
	if t.Sampled.IsSet() {
		sampled = t.Sampled.Val
	}
	out.Sampled = &sampled

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
	out.Timestamp = reqTime
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
			if t.Context.Response.Headers.IsSet() {
				response.Headers = t.Context.Response.Headers.Val.Clone()
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
		if len(t.Context.Custom) > 0 {
			custom := model.Custom(t.Context.Custom.Clone())
			out.Custom = &custom
		}
	}
	if experimental {
		out.Experimental = t.Experimental.Val
	}
}
