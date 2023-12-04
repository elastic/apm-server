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

package beater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/kibana"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/createdatastream"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// checkIndexTemplatesInstalled checks if the APM index templates are installed by querying the
// APM integration status via Kibana, or by attempting to create a data stream via Elasticsearch,
// returning nil if and only if it is installed.
func checkIndexTemplatesInstalled(
	ctx context.Context,
	kibanaClient *kibana.Client,
	esClient *elasticsearch.Client,
	namespace string,
	logger *logp.Logger,
) (err error) {
	defer func() {
		if err != nil {
			// We'd like to include some remediation actions when the APM Integration isn't installed.
			err = &actionableError{
				Err:         err,
				Name:        "apm integration installed",
				Remediation: "please install the apm integration: https://ela.st/apm-integration-quickstart",
			}
		}
	}()
	if esClient != nil {
		installed, err := checkCreateDataStream(ctx, esClient, namespace)
		if err != nil {
			return fmt.Errorf("error checking Elasticsearch index template setup: %w", err)
		}
		if !installed {
			return errors.New("index templates not installed")
		}
		return nil
	}
	if kibanaClient != nil {
		installed, err := checkIntegrationInstalled(ctx, kibanaClient, logger)
		if err != nil {
			// We only return the Kibana error if we have no Elasticsearch client,
			// as we may not have sufficient privileges to query the Fleet API.
			if esClient == nil {
				return fmt.Errorf("error querying Kibana for integration package status: %w", err)
			}
		}
		if !installed {
			// We were able to query Kibana, but the package is not yet installed.
			return errors.New("integration package not yet installed")
		}
	}
	return nil
}

// checkIntegrationInstalledKibana checks if the APM integration package
// is installed by querying Kibana.
func checkIntegrationInstalled(ctx context.Context, kibanaClient *kibana.Client, logger *logp.Logger) (bool, error) {
	resp, err := kibanaClient.Send(ctx, "GET", "/api/fleet/epm/packages/apm", nil, nil, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("unexpected HTTP status: %s (%s)", resp.Status, bytes.TrimSpace(body))
	}
	var result struct {
		Response struct {
			Status string `json:"status"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, errors.Wrap(err, "error decoding integration package response")
	}
	logger.Infof("integration package status: %s", result.Response.Status)
	return result.Response.Status == "installed", nil
}

// checkCreateDataStream attempts to create a traces-apm-<namespace> data stream,
// returning an error if it could not be created. This will fail if there is no
// index template matching the pattern.
func checkCreateDataStream(ctx context.Context, esClient *elasticsearch.Client, namespace string) (bool, error) {
	if _, err := createdatastream.New(esClient).Name("traces-apm-" + namespace).Do(ctx); err != nil {
		var esError *types.ElasticsearchError
		if errors.As(err, &esError) {
			cause := esError.ErrorCause
			if cause.Type == "resource_already_exists_exception" {
				return true, nil
			}
			if cause.Reason != nil && strings.HasPrefix(*cause.Reason, "no matching index template") {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}
