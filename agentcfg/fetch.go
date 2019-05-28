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

package agentcfg

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/convert"
	"github.com/elastic/beats/libbeat/kibana"
)

var (
	endpoint   = "/api/apm/settings/cm/search"
	minVersion = common.Version{Major: 7, Minor: 3}
)

func Fetch(kb *kibana.Client, q Query, err error) (map[string]interface{}, string, error) {
	var doc Doc
	resultBytes, err := request(kb, convert.ToReader(q), err)
	err = convert.FromBytes(resultBytes, &doc, err)
	return doc.Source.Settings, doc.Id, err
}

func request(kb *kibana.Client, r io.Reader, err error) ([]byte, error) {

	if err != nil {
		return nil, err
	}

	if kb == nil {
		return nil, errors.New("No configured Kibana Client: provide apm-server.kibana.* settings")
	}

	if version := kb.GetVersion(); version.LessThan(&minVersion) {
		return nil, errors.New(fmt.Sprintf("Needs Kibana version %s or higher", minVersion.String()))
	}

	resp, err := kb.Send(http.MethodPost, endpoint, nil, nil, r)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New(string(result))
	}

	return result, err
}
