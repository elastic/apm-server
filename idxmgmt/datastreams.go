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

package idxmgmt

import (
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/fmtstr"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/outil"

	"github.com/elastic/apm-server/datastreams"
)

type dataStreamsSupporter struct{}

// BuildSelector returns an outputs.IndexSelector which routes events through
// to data streams based on well-defined data_stream.* fields in events.
func (dataStreamsSupporter) BuildSelector(*common.Config) (outputs.IndexSelector, error) {
	fmtstr, err := fmtstr.CompileEvent(datastreams.IndexFormat)
	if err != nil {
		return nil, err
	}
	expr, err := outil.FmtSelectorExpr(fmtstr, "", outil.SelectorLowerCase)
	if err != nil {
		return nil, err
	}
	return outil.MakeSelector(expr), nil
}

// Enabled always returns false, indicating that this idxmgmt.Supporter does
// not setting up templates or ILM policies.
func (dataStreamsSupporter) Enabled() bool {
	return false
}

// Manager returns a no-op idxmgmt.Manager.
func (dataStreamsSupporter) Manager(client idxmgmt.ClientHandler, assets idxmgmt.Asseter) idxmgmt.Manager {
	return dataStreamsManager{}
}

type dataStreamsManager struct{}

// VerifySetup always returns true and an empty string, to avoid logging
// duplicate warnings.
func (dataStreamsManager) VerifySetup(template, ilm idxmgmt.LoadMode) (bool, string) {
	// Just return true to avoid logging warnings. We'll error out in Setup.
	return true, ""
}

// Setup will always return an error, in response to manual setup (i.e. `apm-server setup`).
func (dataStreamsManager) Setup(template, ilm idxmgmt.LoadMode) error {
	return errors.New("index setup must be performed externally when using data streams, by installing the 'apm' integration package")
}
