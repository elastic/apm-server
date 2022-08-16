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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

// APMServer represents a wrapped exec CMD for APMServer process
type APMServer struct {
	apmHost string
	cmd     *apmservertest.ServerCmd
	mu      sync.Mutex
}

// GetAPMServer returns an APMServer CMD with the required args
func GetAPMServer(ctx context.Context, esHost, apmHost string) *APMServer {
	args := []string{
		"--strict.perms=false",
		"-E", fmt.Sprintf("logging.level=%s", gencorporaConfig.LoggingLevel),
		"-E", "logging.to_stderr=true",
		"-E", "apm-server.data_streams.wait_for_integration=false",
		"-E", fmt.Sprintf("apm-server.host=%s", apmHost),
		"-E", fmt.Sprintf("output.elasticsearch.hosts=['%s']", esHost),
	}

	return &APMServer{
		apmHost: apmHost,
		cmd:     apmservertest.ServerCommand(ctx, "run", args...),
	}
}

// StreamLogs streams logs from the APMServer process configured to log
// all logging output to stderr.
//
// Logs are written to the standard logger. The resources acquired will
// be closed on calling Stop for the APMServer or when the provided context
// is done. StreamLogs must be called before Start is called and only one
// active streaming is allowed.
func (s *APMServer) StreamLogs(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	reader, err := s.cmd.StderrPipe()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	logsCh := make(chan string)
	go func() {
		defer cancel()

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			logsCh <- scanner.Text()
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case logLine := <-logsCh:
				log.Println(logLine)
			}
		}
	}()

	return nil
}

// Start starts the APMServer command using `run` subcommand
func (s *APMServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cmd.Start()
}

// WaitForPublishReady waits for APM-Server to be in publish ready state
// or for context to be canceled
func (s *APMServer) WaitForPublishReady(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ready, _ := s.isPublishReady()
			if ready {
				return nil
			}
		}
	}
}

// Stop sends interrupt signal to the APMServer process, if interrupt
// fails then a kill signal is sent. On successful interrupt Stop waits
// for the process to exit after cleanup.
func (s *APMServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cmd.Cmd.Process.Signal(os.Interrupt); err != nil {
		return s.cmd.Cmd.Process.Kill()
	}

	return s.cmd.Wait()
}

type apmResp struct {
	BuildDate    string `json:"build_date"`
	BuildSHA     string `json:"build_sha"`
	PublishReady bool   `json:"publish_ready"`
	Version      string `json:"version"`
}

func (s *APMServer) isPublishReady() (bool, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/", s.apmHost))
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	var apmResp apmResp
	if err := json.NewDecoder(resp.Body).Decode(&apmResp); err != nil {
		return false, err
	}

	return apmResp.PublishReady, nil
}
