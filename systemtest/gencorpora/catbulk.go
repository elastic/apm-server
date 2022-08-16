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

package gencorpora

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esutil"
)

// CatBulkServer wraps http server and a listener to listen
// for ES requests on any available port
type CatBulkServer struct {
	listener net.Listener
	server   *http.Server
	Addr     string

	writer io.WriteCloser
}

// GetCatBulkServer returns a HTTP Server which can serve as a
// fake ES server writing the response of the bulk request to the
// provided writer. Writes to the provided writer must be thread safe.
func GetCatBulkServer() (*CatBulkServer, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	writer, err := getWriter(gencorporaConfig.WritePath)
	if err != nil {
		return nil, err
	}

	addr := listener.Addr().String()
	return &CatBulkServer{
		listener: listener,
		Addr:     addr,
		server: &http.Server{
			Addr:    addr,
			Handler: handleReq(writer),
		},
		writer: writer,
	}, nil
}

// Start starts the fake ES server on a listener.
func (s *CatBulkServer) Start() error {
	if err := s.server.Serve(s.listener); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop tries to gracefully stop the underlying HTTP server
func (s *CatBulkServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer s.writer.Close()

	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func handleReq(writer io.Writer) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch req.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cluster_uuid": "cat_bulk"}`))
		case http.MethodPost:
			reader := req.Body
			defer req.Body.Close()

			if encoding := req.Header.Get("Content-Encoding"); encoding == "gzip" {
				var err error
				reader, err = gzip.NewReader(reader)
				if err != nil {
					log.Println("failed to read request body", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}

			mockResp := esutil.BulkIndexerResponse{}
			scanner := bufio.NewScanner(reader)
			for count := 0; scanner.Scan(); count++ {
				fmt.Fprintln(writer, scanner.Text())
				// create mock bulk response considering all request items
				// exist with action_and_metadata followed by source line
				if count%2 == 0 {
					item := map[string]esutil.BulkIndexerResponseItem{
						"action": {Status: http.StatusOK},
					}
					mockResp.Items = append(mockResp.Items, item)
				}
			}

			resp, err := json.Marshal(mockResp)
			if err != nil {
				log.Println("failed to encode response to JSON", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func getWriter(writePath string) (io.WriteCloser, error) {
	if writePath == "" {
		return os.Stdout, nil
	}
	return os.Create(filepath.Join(writePath))
}
