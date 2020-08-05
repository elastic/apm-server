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

package apmservertest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"
	"go.uber.org/zap/zapcore"
)

// Server is an APM server listening on a system-chosen port on the local
// loopback interface, for use in end-to-end tests.
type Server struct {
	// Config holds configuration for apm-server, which will be passed
	// to the apm-server command.
	//
	// Config will be initialised with DefaultConfig, and may be changed
	// any time until Start is called.
	Config Config

	// Dir is the working directory of the server.
	//
	// If Dir is empty when Start is called on an unstarted server,
	// then it will be set to a temporary directory, in which an
	// empty apm-server.yml, and the pipeline definition, will be created.
	// The temporary directory will be removed when the server is closed.
	Dir string

	// Logs provides access to the apm-server log entries.
	Logs LogEntries

	// Stderr holds the stderr for apm-server, excluding logging.
	Stderr io.ReadCloser

	// URL holds the base URL for Elastic APM agents, in the form
	// http://ipaddr:port with no trailing slash.
	URL string

	// JaegerGRPCAddr holds the address for the Jaeger gRPC server, if enabled.
	JaegerGRPCAddr string

	// JaegerHTTPURL holds the base URL for Jaeger HTTP, if enabled.
	JaegerHTTPURL string

	tb   testing.TB
	args []string
	cmd  *ServerCmd
}

// NewServer returns a started Server, passings args to the apm-server command.
// The server's Close method will be called when the test ends.
func NewServer(tb testing.TB, args ...string) *Server {
	s := NewUnstartedServer(tb, args...)
	if err := s.Start(); err != nil {
		tb.Fatal(err)
	}
	return s
}

// NewUnstartedServer returns an unstarted Server, passing args to the
// apm-server command.
func NewUnstartedServer(tb testing.TB, args ...string) *Server {
	return &Server{
		Config: DefaultConfig(),
		tb:     tb,
		args:   args,
	}
}

// Start starts a server from NewUnstartedServer, waiting for it to start
// listening for requests.
//
// Start will have set s.URL upon a successful return.
func (s *Server) Start() error {
	if s.URL != "" {
		panic("Server already started")
	}
	s.Logs.init()

	cfgargs, err := configArgs(s.Config, map[string]interface{}{
		// These are config attributes that we always specify,
		// as the testing framework relies on them being set.
		"logging.json":              true,
		"logging.to_stderr":         true,
		"apm-server.expvar.enabled": true,
		"apm-server.host":           "127.0.0.1:0",
	})
	if err != nil {
		return err
	}
	args := append(cfgargs, s.args...)
	args = append(args, "--path.home", ".") // working directory, s.Dir

	s.cmd = ServerCommand("run", args...)
	s.cmd.Dir = s.Dir

	// This speeds up tests by forcing the self-instrumentation
	// event streams to be closed after 100ms. This is only necessary
	// because processor/stream waits for the stream to be closed
	// before the last batch is processed.
	//
	// TODO(axw) remove this once the server processes batches without
	// waiting for the stream to be closed.
	s.cmd.Env = append(os.Environ(), "ELASTIC_APM_API_REQUEST_TIME=100ms")

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := s.cmd.Start(); err != nil {
		stderr.Close()
		return err
	}
	s.Dir = s.cmd.Dir
	s.tb.Cleanup(func() { s.Close() })

	logfile := createLogfile(s.tb, "apm-server")
	s.tb.Cleanup(func() {
		if s.tb.Failed() {
			s.tb.Logf("log file: %s", logfile.Name())
		}
	})

	// Write the apm-server command line to the top of the log file.
	s.printCmdline(logfile, args)
	go s.consumeStderr(io.TeeReader(stderr, logfile))

	logs := s.Logs.Iterator()
	defer logs.Close()
	if err := s.waitUntilListening(logs); err != nil {
		return err
	}
	return nil
}

func (s *Server) printCmdline(w io.Writer, args []string) {
	var buf bytes.Buffer
	fmt.Fprint(&buf, "# Running apm-server\n")
	for i := 0; i < len(args); i += 2 {
		fmt.Fprintf(&buf, "# \t")
		if args[i] == "-E" && i+1 < len(args) {
			fmt.Fprintf(&buf, "%s %s\n", args[i], args[i+1])
		} else {
			fmt.Fprintf(&buf, "%s\n", strings.Join(args[i:], " "))
			break
		}
	}
	if _, err := buf.WriteTo(w); err != nil {
		s.tb.Fatal(err)
	}
}

func (s *Server) waitUntilListening(logs *LogEntryIterator) error {
	var (
		elasticHTTPListeningAddr string
		jaegerGRPCListeningAddr  string
		jaegerHTTPListeningAddr  string
	)

	prefixes := map[string]*string{"Listening on": &elasticHTTPListeningAddr}
	if s.Config.Jaeger != nil {
		if s.Config.Jaeger.GRPCEnabled {
			prefixes["Listening for Jaeger gRPC requests on"] = &jaegerGRPCListeningAddr
		}
		if s.Config.Jaeger.HTTPEnabled {
			prefixes["Listening for Jaeger HTTP requests on"] = &jaegerHTTPListeningAddr
		}
	}

	for entry := range logs.C() {
		if entry.Level != zapcore.InfoLevel {
			continue
		}
		sep := strings.LastIndex(entry.Message, ": ")
		if sep == -1 {
			continue
		}
		prefix, addr := entry.Message[:sep], strings.TrimSpace(entry.Message[sep+1:])
		paddr, ok := prefixes[prefix]
		if !ok {
			continue
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return fmt.Errorf("invalid listening address %q: %w", addr, err)
		}
		*paddr = addr
		delete(prefixes, prefix)
		if len(prefixes) == 0 {
			break
		}
	}

	if len(prefixes) == 0 {
		s.URL = makeHTTPURLString(elasticHTTPListeningAddr)
		if s.Config.Jaeger != nil {
			s.JaegerGRPCAddr = jaegerGRPCListeningAddr
			if s.Config.Jaeger.HTTPEnabled {
				s.JaegerHTTPURL = makeHTTPURLString(jaegerHTTPListeningAddr)
			}
		}
		return nil
	}

	// Didn't find message, server probably exited...
	if err := s.Close(); err != nil {
		if err, ok := err.(*exec.ExitError); ok && err != nil {
			stderr, _ := ioutil.ReadAll(s.Stderr)
			err.Stderr = stderr
		}
		return err
	}
	return errors.New("server exited cleanly without logging expected startup message")
}

// consumeStderr consumes the apm-server process's stderr, recording
// log entries. After any errors occur decoding log entries, remaining
// stderr is available through s.Stderr.
func (s *Server) consumeStderr(procStderr io.Reader) {
	stderrPipeReader, stderrPipeWriter := io.Pipe()
	s.Stderr = stderrPipeReader

	type logEntry struct {
		Timestamp logpTimestamp
		Level     zapcore.Level
		Logger    string
		Caller    string
		Message   string
	}

	decoder := json.NewDecoder(procStderr)
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			break
		}
		var entry logEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			break
		}
		var fields map[string]interface{}
		if err := json.Unmarshal(raw, &fields); err != nil {
			break
		}
		delete(fields, "timestamp")
		delete(fields, "level")
		delete(fields, "logger")
		delete(fields, "caller")
		delete(fields, "message")
		s.Logs.add(LogEntry{
			Timestamp: time.Time(entry.Timestamp),
			Logger:    entry.Logger,
			Level:     entry.Level,
			Caller:    entry.Caller,
			Message:   entry.Message,
			Fields:    fields,
		})
	}
	s.Logs.close()

	// Send the remaining stderr to s.Stderr.
	procStderr = io.MultiReader(decoder.Buffered(), procStderr)
	_, err := io.Copy(stderrPipeWriter, procStderr)
	stderrPipeWriter.CloseWithError(err)
}

// Close shuts down the server gracefully if possible, and forcefully otherwise.
//
// Close must be called in order to clean up any resources created for running
// the server.
func (s *Server) Close() error {
	if s.cmd != nil {
		if err := interruptProcess(s.cmd.Process); err != nil {
			s.cmd.Process.Kill()
		}
	}
	return s.Wait()
}

// Wait waits for the server to exit.
//
// Wait waits up to 10 seconds for the process's stderr to be closed,
// and then waits for the process to exit.
func (s *Server) Wait() error {
	if s.cmd == nil {
		return errors.New("apm-server not started")
	}

	logs := s.Logs.Iterator()
	defer logs.Close()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case _, ok := <-logs.C():
			if !ok {
				return s.cmd.Wait()
			}
		case <-deadline:
			return s.cmd.Wait()
		}
	}
}

// Tracer returns a new apm.Tracer, configured with the server's URL and secret
// token if any. This must only be called after the server has been started.
//
// The Tracer will be closed when the test ends.
func (s *Server) Tracer() *apm.Tracer {
	serverURL, err := url.Parse(s.URL)
	if err != nil {
		s.tb.Fatal(err)
	}
	httpTransport, err := transport.NewHTTPTransport()
	if err != nil {
		s.tb.Fatal(err)
	}
	httpTransport.SetServerURL(serverURL)
	httpTransport.SetSecretToken(s.Config.SecretToken)
	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		Transport: httpTransport,
	})
	if err != nil {
		s.tb.Fatal(err)
	}
	s.tb.Cleanup(tracer.Close)
	return tracer
}

// GetExpvar queries the server's /debug/vars endpoint, parsing the response
// into an Expvar structure.
func (s *Server) GetExpvar() *Expvar {
	resp, err := http.Get(s.URL + "/debug/vars")
	if err != nil {
		s.tb.Fatal(err)
	}
	defer resp.Body.Close()
	expvar, err := decodeExpvar(resp.Body)
	if err != nil {
		s.tb.Fatal(err)
	}
	return expvar
}

func makeHTTPURLString(host string) string {
	u := url.URL{Scheme: "http", Host: host}
	return u.String()
}
