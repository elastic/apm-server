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

package eventhandler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// Transport sends the contents of a reader to a remote APM Server.w
type Transport struct {
	client        *http.Client
	intakeHeaders http.Header
	intakeV2URL   string
}

// NewTransport initializes a new ReplayTransport.
func NewTransport(c *http.Client, srvURL, token string) *Transport {
	intakeHeaders := make(http.Header)
	intakeHeaders.Set("Content-Encoding", "deflate")
	intakeHeaders.Set("Content-Type", "application/x-ndjson")
	intakeHeaders.Set("Transfer-Encoding", "chunked")
	intakeHeaders.Set("Authorization", "Bearer "+token)
	return &Transport{
		client:        c,
		intakeV2URL:   srvURL + `/intake/v2/events`,
		intakeHeaders: intakeHeaders,
	}
}

// SendV2Events sends the reader contents to `/intake/v2/events` as a batch.
func (t *Transport) SendV2Events(ctx context.Context, r io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "POST", t.intakeV2URL, r)
	if err != nil {
		return err
	}
	// Since the ContentLength will be automatically set on `bytes.Reader`,
	// set it to `-1` just like the agents would.
	req.ContentLength = -1
	req.Header = t.intakeHeaders
	return t.sendEvents(req, r)
}

func (t *Transport) sendEvents(req *http.Request, r io.Reader) error {
	res, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		return nil
	}
	b, err := ioutil.ReadAll(res.Body)
	msg := fmt.Sprintf("unexpected apm server response %d", res.StatusCode)
	if err != nil {
		return errors.New(msg)
	}
	return fmt.Errorf(msg+": %s", string(b))
}
