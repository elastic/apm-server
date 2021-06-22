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

package kibana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/common"
)

// TODO: Get this value from Oliver
// https://github.com/elastic/kibana/issues/100657
const kibanaConfigUploadPath = "/apm/fleet/apm_server_settings"

// SendConfig marshals and uploads the provided config to kibana using the
// provided ConnectingClient. It retries until its context has been canceled or
// the upload succeeds.
func SendConfig(ctx context.Context, client Client, conf *common.Config) error {
	// configuration options are already flattened (dotted)
	// any credentials for ES and Kibana are removed
	flat, err := flattenAndClean(conf)
	if err != nil {
		return err
	}
	b, err := json.Marshal(flat)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := client.Send(ctx, http.MethodPost, kibanaConfigUploadPath, nil, nil, bytes.NewReader(b))
		if err != nil {
			if errors.Is(err, errNotConnected) {
				// Not connected to kibana, wait and try again.
				time.Sleep(15 * time.Second)
				continue
			}

			// Are there other kinds of recoverable errors?
			return err
		}
		// TODO: What sort of response will we get?
		if resp.StatusCode > http.StatusOK {
			return fmt.Errorf("bad response %s", resp.Status)
		}

		return nil
	}
}

func flattenAndClean(conf *common.Config) (map[string]interface{}, error) {
	m := common.MapStr{}
	if err := conf.Unpack(m); err != nil {
		return nil, err
	}
	flat := m.Flatten()
	out := make(common.MapStr, len(flat))
	for k, v := range flat {
		// remove if elasticsearch is NOT in the front position?
		// *.elasticsearch.* according to axw
		if strings.Contains(k, "elasticsearch") {
			continue
		}
		if strings.Contains(k, "kibana") {
			continue
		}
		if strings.HasPrefix(k, "instrumentation") {
			continue
		}
		if strings.HasPrefix(k, "apm-server.ssl.") {
			// Following ssl related settings need to be synced:
			// apm-server.ssl.enabled
			// apm-server.ssl.certificate
			// apm-server.ssl.key
			switch k[15:] {
			case "enabled", "certificate", "key":
			default:
				continue
			}
		}
		out[k] = v
	}
	return out, nil
}
