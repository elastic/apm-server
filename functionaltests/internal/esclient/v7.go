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

package esclient

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

type IndicesDocCount map[string]int

func (c *Client) APMDocCountV7(ctx context.Context) (IndicesDocCount, error) {
	indicesToCheck := []string{
		"apm-*-transaction-*", "apm-*-span-*", "apm-*-error-*", "apm-*-metric-*",
		"apm-*-profile-*",
		"apm-*-onboarding-*",
	}

	getIndexCount := func(index string) (docCountV7, error) {
		resp, err := c.es.
			Count().
			Index(index).
			FilterPath("count").
			Perform(ctx)
		if err != nil {
			return docCountV7{}, fmt.Errorf("cannot get count for %s: %w", indicesToCheck[0], err)
		}

		if resp.StatusCode > http.StatusOK {
			return docCountV7{}, fmt.Errorf("count request for %s returned unexpected status code: %d", indicesToCheck[0], resp.StatusCode)
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return docCountV7{}, fmt.Errorf("cannot read response body for %s: %w", resp.Request.URL.Path, err)
		}
		defer resp.Body.Close()

		var dc docCountV7
		err = json.Unmarshal(b, &dc)
		if err != nil {
			return docCountV7{}, fmt.Errorf("cannot unmarshal JSON response for %s: %w", resp.Request.URL.Path, err)
		}
		return dc, nil
	}

	count := IndicesDocCount{}
	var errs []error
	for _, idx := range indicesToCheck {
		dc, err := getIndexCount(idx)
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
