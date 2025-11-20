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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/transport"
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

	// CertDir is the directory of the TLS certificates.
	//
	// If Dir is empty the current directory is used
	CertDir string

	// Log holds an optional io.Writer to which the process's Stderr will
	// be written, in addition to being available through the Server.Logs
	// field.
	//
	// Log is set by NewServerTB and NewUnstartedServerTB, and will be nil
	// for calls to NewUnstartedServer. Callers of NewUnstartedServer may
	// set Log prior to calling Start.
	Log io.Writer

	// BeatUUID will be populated with the server's Beat UUID after Start
	// returns successfully. This can be used to search for documents
	// corresponding to this test server instance.
	BeatUUID string

	// Version will be populated with the servers' version number after
	// Start returns successfully.
	Version string

	// Logs provides access to the apm-server log entries.
	Logs LogEntries

	// Stderr holds the stderr for apm-server, excluding logging.
	Stderr io.ReadCloser

	// URL holds the base URL for Elastic APM agents, in the form
	// http[s]://ipaddr:port with no trailing slash.
	URL string

	// TLS is optional TLS client configuration, populated with a new config
	// after TLS is started.
	TLS *tls.Config

	// EventMetadataFilter holds an optional EventMetadataFilter, which
	// can modify event metadata before it is sent to the server.
	//
	// New(Unstarted)Server sets a default filter which removes or
	// replaces environment-specific properties such as host name,
	// container ID, etc., to enable repeatable tests across different
	// test environments.
	EventMetadataFilter EventMetadataFilter

	args []string
	cmd  *ServerCmd

	mu      sync.Mutex
	tracers []*apm.Tracer
	closeCh chan struct{}
}

// NewServerTB returns a started Server, passings args to the apm-server command.
// The server's Close method will be called when the test ends, and logs will be
// written under apm-server/systemtest/logs/<test-name>/.
func NewServerTB(tb testing.TB, args ...string) *Server {
	s := NewUnstartedServerTB(tb, args...)
	if err := s.Start(); err != nil {
		tb.Fatal(err)
	}
	return s
}

// NewUnstartedServerTB returns an unstarted Server, passing args to the apm-server
// command. The server's Close method will be called when the test ends, and logs
// will be written under apm-server/systemtest/logs/<test-name>/.
func NewUnstartedServerTB(tb testing.TB, args ...string) *Server {
	s := NewUnstartedServer(args...)
	s.CertDir = tb.TempDir()
	logfile := createLogfile(tb, "apm-server")
	s.Log = logfile
	tb.Cleanup(func() {
		defer logfile.Close()
		if tb.Failed() {
			tb.Logf("log file: %s", logfile.Name())
		}

		if tb.Failed() {
			b, err := io.ReadAll(logfile)
			if err != nil {
				tb.Fatal(err)
			}

			tb.Log(string(b))
		}

		// Call the server's Close method in a background goroutine,
		// and wait for up to 10 seconds for it to complete.
		errc := make(chan error)
		go func() { errc <- s.Close() }()
		select {
		case err := <-errc:
			if err != nil {
				tb.Error(err)
			}
			close(errc)
		case <-time.After(10 * time.Second):
			// Channel receive on errc never happened. Start up a
			// goroutine to receive on errc and then clean up the
			// associated resources.
			go func() { <-errc; close(errc) }()
		}
	})
	return s
}

// NewUnstartedServer returns an unstarted Server, passing args to the apm-server
// command. The server's Close method must be called to clean up any resources
// created by Start.
func NewUnstartedServer(args ...string) *Server {
	return &Server{
		Config:              DefaultConfig(),
		EventMetadataFilter: DefaultMetadataFilter{},
		args:                args,
		closeCh:             make(chan struct{}),
	}
}

// Start starts a server from NewUnstartedServer, waiting for it to start
// listening for requests.
//
// Start will have set s.URL upon a successful return.
func (s *Server) Start() error {
	return s.start(false)
}

func (s *Server) StartTLS() error {
	return s.start(true)
}

func (s *Server) start(tls bool) error {
	if s.URL != "" {
		panic("Server already started")
	}
	s.Logs.init()

	extra := map[string]interface{}{
		// These are config attributes that we always specify,
		// as the testing framework relies on them being set.
		"logging.level":             "debug",
		"logging.to_stderr":         true,
		"apm-server.expvar.enabled": true,
		"apm-server.host":           "127.0.0.1:0",
	}
	if tls {
		certPath, keyPath, caCertPath, err := s.initTLS()
		if err != nil {
			panic(err)
		}
		extra["apm-server.ssl.certificate"] = certPath
		extra["apm-server.ssl.key"] = keyPath
		if s.Config.TLS != nil && s.Config.TLS.ClientAuthentication != "" {
			extra["apm-server.ssl.certificate_authorities"] = []string{caCertPath}
		}
	}
	cfgargs, err := configArgs(s.Config, extra)
	if err != nil {
		return err
	}
	args := append(cfgargs, s.args...)
	args = append(args, "--path.home", ".") // working directory, s.Dir

	s.cmd = ServerCommand(context.Background(), "run", args...)
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

	// Consume the process's stderr.
	var stderrReader io.Reader = stderr
	if s.Log != nil {
		// Write the apm-server command line to the top of the log.
		s.printCmdline(s.Log, args)
		stderrReader = io.TeeReader(stderrReader, s.Log)
	}

	stderrPipeReader, stderrPipeWriter := io.Pipe()
	s.Stderr = stderrPipeReader

	go s.consumeStderr(stderrReader, stderrPipeWriter)

	logs := s.Logs.Iterator()
	defer logs.Close()
	if err := s.waitUntilListening(tls, logs); err != nil {
		return err
	}
	return nil
}

func (s *Server) initTLS() (serverCertPath, serverKeyPath, caCertPath string, _ error) {
	serverCertPath, serverKeyPath, err := generateCerts(s.CertDir, true, x509.ExtKeyUsageServerAuth, "127.0.0.1", "::1")
	// Load a self-signed server certificate for testing TLS encryption.
	serverCertBytes, err := os.ReadFile(serverCertPath)
	if err != nil {
		return "", "", "", err
	}
	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(serverCertBytes) {
		panic("failed to add CA certificate to cert pool")
	}

	clientCertPath, clientKeyPath, err := generateCerts(s.CertDir, false, x509.ExtKeyUsageClientAuth, "")
	if err != nil {
		return "", "", "", err
	}

	// Load a self-signed client certificate for testing TLS client certificate auth.
	clientCertBytes, err := os.ReadFile(clientCertPath)
	if err != nil {
		return "", "", "", err
	}
	clientKeyBytes, err := os.ReadFile(clientKeyPath)
	if err != nil {
		return "", "", "", err
	}
	clientCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	if err != nil {
		return "", "", "", err
	}

	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certpool,
	}
	return serverCertPath, serverKeyPath, clientCertPath, nil
}

func generateCerts(dir string, ca bool, keyUsage x509.ExtKeyUsage, hosts ...string) (string, string, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("Failed to generate serial number: %w", err)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Org"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{keyUsage},
		BasicConstraintsValid: true,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		}
	}

	if ca {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate client key: %w", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshal private key: %w", err)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, clientKey.Public(), clientKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}
	certOut, err := os.CreateTemp(dir, "client_cert.pem")
	if err != nil {
		return "", "", fmt.Errorf("failed to open client_cert.pem for writing: %w", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to client_cert.pem: %w", err)
	}
	if err := certOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing client_cert.pem: %w", err)
	}

	keyOut, err := os.CreateTemp(dir, "client_key.pem")
	if err != nil {
		return "", "", fmt.Errorf("failed to open client_key.pem for writing: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write data to client_key.pem: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		return "", "", fmt.Errorf("error closing client_key.pem: %w", err)
	}

	return certOut.Name(), keyOut.Name(), nil
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
		panic(err)
	}
}

func (s *Server) waitUntilListening(tls bool, logs *LogEntryIterator) error {
	// First wait for the Beat UUID and server version to be logged.
	for entry := range logs.C() {
		if entry.Level != zapcore.InfoLevel || (entry.Message != "Beat info" && entry.Message != "Build info") {
			continue
		}
		systemInfo, ok := entry.Fields["system_info"].(map[string]interface{})
		if !ok {
			continue
		}
		for k, info := range systemInfo {
			switch k {
			case "beat":
				beatInfo := info.(map[string]interface{})
				s.BeatUUID = beatInfo["uuid"].(string)
			case "build":
				buildInfo := info.(map[string]interface{})
				s.Version = buildInfo["version"].(string)
			}
		}
		if s.BeatUUID != "" && s.Version != "" {
			break
		}
	}

	var elasticHTTPListeningAddr string
	for entry := range logs.C() {
		if entry.Level != zapcore.InfoLevel {
			continue
		}
		sep := strings.LastIndex(entry.Message, ": ")
		if sep == -1 {
			continue
		}
		prefix, addr := entry.Message[:sep], strings.TrimSpace(entry.Message[sep+1:])
		if prefix != "Listening on" {
			continue
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return fmt.Errorf("invalid listening address %q: %w", addr, err)
		}
		elasticHTTPListeningAddr = addr
		break
	}

	if elasticHTTPListeningAddr != "" {
		urlScheme := "http"
		if tls {
			urlScheme = "https"
		}
		s.URL = (&url.URL{Scheme: urlScheme, Host: elasticHTTPListeningAddr}).String()
		return nil
	}

	// Didn't find message, server probably exited...
	if err := s.Close(); err != nil {
		if err, ok := err.(*exec.ExitError); ok && err != nil {
			stderr, _ := io.ReadAll(s.Stderr)
			err.Stderr = stderr
		}
		return err
	}
	return errors.New("server exited cleanly without logging expected startup message")
}

// consumeStderr consumes the apm-server process's stderr, recording
// log entries. After any errors occur decoding log entries, remaining
// stderr is available through s.Stderr.
func (s *Server) consumeStderr(procStderr io.Reader, out *io.PipeWriter) {
	type logEntry struct {
		Timestamp logpTimestamp `json:"@timestamp"`
		Message   string        `json:"message"`
		Level     zapcore.Level `json:"log.level"`
		Logger    string        `json:"log.logger"`
		Origin    struct {
			File string `json:"file.name"`
			Line int    `json:"file.line"`
		} `json:"log.origin"`
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
		delete(fields, "@timestamp")
		delete(fields, "log.level")
		delete(fields, "log.logger")
		delete(fields, "log.origin")
		delete(fields, "message")
		s.Logs.add(LogEntry{
			Timestamp: time.Time(entry.Timestamp),
			Logger:    entry.Logger,
			Level:     entry.Level,
			File:      entry.Origin.File,
			Line:      entry.Origin.Line,
			Message:   entry.Message,
			Fields:    fields,
		})
	}
	s.Logs.close()

	// Send the remaining stderr to s.Stderr.
	procStderr = io.MultiReader(decoder.Buffered(), procStderr)
	_, err := io.Copy(out, procStderr)
	out.CloseWithError(err)
}

// Close shuts down the server gracefully if possible, and forcefully otherwise.
//
// Close must be called in order to clean up any resources created for running
// the server. Calling Close on an unstarted server is a no-op.
func (s *Server) Close() error {
	select {
	case <-s.closeCh:
		return nil
	default:
		close(s.closeCh)
	}

	if s.cmd == nil {
		return nil
	}
	s.closeTracers()
	if s.cmd.Process == nil {
		return errors.New("apm server process not started")
	}
	if err := interruptProcess(s.cmd.Process); err != nil {
		s.cmd.Process.Kill()
	}
	if err := s.Wait(); err != nil {
		return err
	}
	// close stderr so that the consumeStderr goroutine
	// exits
	s.Stderr.Close()
	return nil
}

func (s *Server) closeTracers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tracer := range s.tracers {
		tracer.Close()
	}
	s.tracers = nil
}

// Kill forcefully shuts down the server.
func (s *Server) Kill() error {
	if s.cmd != nil {
		s.cmd.Process.Kill()
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
// The Tracer will be closed when the server is closed.
func (s *Server) Tracer() *apm.Tracer {
	serverURL, err := url.Parse(s.URL)
	if err != nil {
		panic(err)
	}
	httpTransport, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{
		ServerURLs:      []*url.URL{serverURL},
		SecretToken:     s.Config.AgentAuth.SecretToken,
		TLSClientConfig: s.TLS,
	})
	if err != nil {
		panic(err)
	}

	opts := apm.TracerOptions{Transport: httpTransport}
	if s.EventMetadataFilter != nil {
		opts.Transport = NewFilteringTransport(httpTransport, s.EventMetadataFilter)
	}
	tracer, err := apm.NewTracerOptions(opts)
	if err != nil {
		panic(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tracers = append(s.tracers, tracer)
	return tracer
}

// GetExpvar queries the server's /debug/vars endpoint, parsing the response
// into an Expvar structure.
func (s *Server) GetExpvar() *Expvar {
	resp, err := http.Get(s.URL + "/debug/vars")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	expvar, err := decodeExpvar(resp.Body)
	if err != nil {
		panic(err)
	}
	return expvar
}

// WaitForPublishReady polls the server's "GET /" endpoint, waiting for it to
// indicate it is in the publish-ready state, or for the context to be canceled.
func (s *Server) WaitForPublishReady(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ready, err := s.isPublishReady()
			if err != nil {
				// Errors are not expected, as the server
				// should be operational by the time Start
				// returns.
				return err
			}
			if ready {
				return nil
			}
		}
	}
}

func (s *Server) isPublishReady() (bool, error) {
	req, err := http.NewRequest(http.MethodGet, s.URL+"/", nil)
	if err != nil {
		return false, err
	}
	if secretToken := s.Config.AgentAuth.SecretToken; secretToken != "" {
		req.Header.Set("Authorization", "Bearer "+secretToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var apmResp struct {
		BuildDate    string `json:"build_date"`
		BuildSHA     string `json:"build_sha"`
		PublishReady bool   `json:"publish_ready"`
		Version      string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apmResp); err != nil {
		return false, err
	}
	return apmResp.PublishReady, nil
}
