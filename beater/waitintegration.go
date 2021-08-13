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
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.elastic.co/apm"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// waitForIntegration waits for the APM integration to be installed by querying Kibana,
// or for the context to be cancelled.
func waitForIntegration(
	ctx context.Context,
	kibanaClient kibana.Client,
	esClient elasticsearch.Client,
	interval time.Duration,
	tracer *apm.Tracer,
	logger *logp.Logger,
) error {
	logger.Info("waiting for integration package to be installed")
	tx := tracer.StartTransaction("wait_for_integration", "init")
	ctx = apm.ContextWithTransaction(ctx, tx)
	var ticker *time.Ticker
	for {
		if ticker == nil {
			// We start the ticker on the first iteration, rather than
			// before the loop, so we don't have to wait for a tick
			// (5 seconds by default) before peforming the first check.
			ticker = time.NewTicker(interval)
			defer ticker.Stop()
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
		if kibanaClient != nil {
			installed, err := checkIntegrationInstalledKibana(ctx, kibanaClient, logger)
			if err != nil {
				logger.Errorf("error querying Kibana for integration package status: %s", err)
			} else {
				if installed {
					return nil
				}
				// We were able to query Kibana, but the package is not yet installed.
				// We should continue querying the package status via Kibana, as it is
				// more authoritative than checking for index template installation.
				continue
			}
		}
		if esClient != nil {
			installed, err := checkIntegrationInstalledElasticsearch(ctx, esClient, logger)
			if err != nil {
				logger.Errorf("error querying Elasticsearch for integration index templates: %s", err)
			} else if installed {
				return nil
			}
		}
	}
}

// checkIntegrationInstalledKibana checks if the APM integration package
// is installed by querying Kibana.
func checkIntegrationInstalledKibana(ctx context.Context, kibanaClient kibana.Client, logger *logp.Logger) (bool, error) {
	resp, err := kibanaClient.Send(ctx, "GET", "/api/fleet/epm/packages/apm", nil, nil, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
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

func checkIntegrationInstalledElasticsearch(ctx context.Context, esClient elasticsearch.Client, logger *logp.Logger) (bool, error) {
	// TODO(axw) generate the list of expected index templates.
	templates := []string{
		"traces-apm",
		"traces-sampled",
		"metrics-apm.app",
		"metrics-apm.internal",
		"logs-apm.error",
	}
	// IndicesGetIndexTemplateRequest accepts a slice of template names,
	// but the REST API expects just one index template name. Query them
	// in parallel.
	g, ctx := errgroup.WithContext(ctx)
	for _, template := range templates {
		template := template // copy for closure
		g.Go(func() error {
			req := esapi.IndicesGetIndexTemplateRequest{Name: []string{template}}
			resp, err := req.Do(ctx, esClient)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.IsError() {
				body, _ := ioutil.ReadAll(resp.Body)
				return fmt.Errorf("unexpected HTTP status: %s (%s)", resp.Status(), bytes.TrimSpace(body))
			}
			return nil
		})
	}
	err := g.Wait()
	return err == nil, err
}
