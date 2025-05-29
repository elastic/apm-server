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

package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type docCountV7 struct {
	Count int `json:"count"`
}

// IndicesDocCount is an easy to assert on format reporting document count
// for indices.
type IndicesDocCount map[string]int

// APMIdxDocCountV7 retrieves the document count per index of all APM indices.
func (c *Client) APMIdxDocCountV7(ctx context.Context) (IndicesDocCount, error) {
	indicesToCheck := []string{
		"apm-*-transaction-*", "apm-*-span-*", "apm-*-error-*", "apm-*-metric-*",
	}

	count := IndicesDocCount{}
	var errs []error
	for _, idx := range indicesToCheck {
		dc, err := c.getDocCountV7(ctx, idx)
		if err != nil {
			errs = append(errs, err)
		}
		count[idx] = dc.Count
	}

	if len(errs) > 0 {
		return IndicesDocCount{}, errors.Join(errs...)
	}
	return count, nil
}

// APMDSDocCountV7 retrieves the document count per data stream of all APM data streams.
func (c *Client) APMDSDocCountV7(ctx context.Context, namespace string) (DataStreamsDocCount, error) {
	dsToCheck := []string{
		"traces-apm-%s", "metrics-apm.internal-%s", "logs-apm.error-%s",
		"metrics-apm.app.opbeans_python-%s", "metrics-apm.app.opbeans_node-%s",
		"metrics-apm.app.opbeans_ruby-%s", "metrics-apm.app.opbeans_go-%s",
	}
	for i, ds := range dsToCheck {
		dsToCheck[i] = fmt.Sprintf(ds, namespace)
	}

	count := DataStreamsDocCount{}
	var errs []error
	for _, ds := range dsToCheck {
		dc, err := c.getDocCountV7(ctx, ds)
		if err != nil {
			errs = append(errs, err)
		}
		count[ds] = dc.Count
	}

	if len(errs) > 0 {
		return DataStreamsDocCount{}, errors.Join(errs...)
	}
	return count, nil
}

func (c *Client) getDocCountV7(ctx context.Context, name string) (docCountV7, error) {
	resp, err := c.es.
		Count().
		Index(name).
		FilterPath("count").
		Perform(ctx)
	if err != nil {
		return docCountV7{}, fmt.Errorf("cannot get count for %s: %w", name, err)
	}

	// If not found, return zero count instead of error.
	if resp.StatusCode == http.StatusNotFound {
		return docCountV7{Count: 0}, nil
	}

	if resp.StatusCode > http.StatusOK {
		return docCountV7{}, fmt.Errorf(
			"count request for %s returned unexpected status code: %d",
			name, resp.StatusCode,
		)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return docCountV7{}, fmt.Errorf(
			"cannot read response body for %s: %w",
			resp.Request.URL.Path, err,
		)
	}
	defer resp.Body.Close()

	var dc docCountV7
	err = json.Unmarshal(b, &dc)
	if err != nil {
		return docCountV7{}, fmt.Errorf(
			"cannot unmarshal JSON response for %s: %w",
			resp.Request.URL.Path, err,
		)
	}
	return dc, nil
}
