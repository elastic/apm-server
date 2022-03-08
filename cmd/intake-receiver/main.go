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

//go:generate bash ../../script/intake-receiver-version.sh
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"go.elastic.co/apm/v2/model"
)

var maxScannerBufSize = 300 * 1024 // APM Server default

func main() {
	// Ignored flags, they are just here to allow the `intake-receiver` to be
	// dropped in as a replacement for APM Server. This means, that all the
	// config options are ignored.
	flag.String("e", "", "apm-server compatibility option")
	flag.String("E", "", "apm-server compatibility option")
	flag.String("httpprof", "", "apm-server compatibility option")

	var host, folder string
	flag.StringVar(&host, "host", ":8200", "port that the server will listen to")
	flag.StringVar(&folder, "folder", "events", "The path where the received intake events will be stored")
	flag.Parse()

	// Create a context that will be cancelled when an interrupt is received.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	if err := os.MkdirAll(folder, 0755); err != nil {
		log.Fatalln(err)
	}
	var agentFileMap fileMap
	defer agentFileMap.m.Range(func(_, v interface{}) bool {
		if f, ok := v.(*syncFile); ok && f != nil {
			f.Close()
			log.Println("closed file", f.Name())
		}
		return true
	})
	rh := requestHandler{
		agentFileMap: &agentFileMap,
		basePath:     folder,
		rootResponse: fmt.Sprintf(`{"publish_ready":true,"version":"%s"}`+"\n", version),
		bufPool:      sync.Pool{New: func() interface{} { return &bytes.Buffer{} }},
		bytesBufPool: sync.Pool{New: func() interface{} { return make([]byte, maxScannerBufSize) }},
	}
	mux := http.NewServeMux()
	mux.Handle("/", rh.rootHandler())
	// TODO(marclop): intake/v2/rum and /intake/v3/rum/events are not supported.
	for _, p := range []string{"/intake/v2/events"} {
		mux.Handle(p, rh.eventHandler())
	}
	srv := http.Server{
		Addr:        host,
		Handler:     mux,
		ReadTimeout: 30 * time.Second,
		BaseContext: func(l net.Listener) context.Context { return ctx },
	}

	go func() {
		<-ctx.Done()
		log.Println("closing http server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Println(err)
		}
	}()

	log.Println("http server listening for requests on", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Println(err)
	}
}

type requestHandler struct {
	agentFileMap *fileMap
	bufPool      sync.Pool
	bytesBufPool sync.Pool
	basePath     string
	rootResponse string
}

func (h requestHandler) rootHandler() http.Handler {
	return logHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			rw.WriteHeader(200)
			rw.Write([]byte(h.rootResponse))
		case "/config/v1/agents":
			// Prevent the APM Agents from logging errors.
			rw.WriteHeader(200)
			rw.Write([]byte(`{}`))
		default:
			http.Error(
				rw,
				fmt.Sprintf(" %s is not implemented", r.URL.Path),
				http.StatusNotImplemented,
			)
		}
	}))
}

func (h requestHandler) eventHandler() http.Handler {
	return logHandler(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			code, err := h.handleRequest(r)
			if err != nil {
				log.Println("failed handling request", code, err.Error())
				http.Error(rw, err.Error(), code)
				return
			}
			rw.WriteHeader(code)
		}),
	)
}

func (h requestHandler) handleRequest(r *http.Request) (int, error) {
	buf := h.bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		h.bufPool.Put(buf)
	}()

	var err error
	body := r.Body
	encoding := r.Header.Get("Content-Encoding")
	switch encoding {
	case "deflate":
		body, err = zlib.NewReader(r.Body)
	case "gzip":
		body, err = gzip.NewReader(r.Body)
	case "":
	default:
		return http.StatusBadRequest, fmt.Errorf(
			"Content-Encoding %s not supported", encoding,
		)
	}
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf(
			"unable to create compressed reader for %s: %v", encoding, err,
		)
	}

	var meta metadata
	if err := h.processBatch(body, buf, &meta); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request: %v", err)
	}
	if meta.IsEmpty() {
		return http.StatusBadRequest, errors.New("agent not found in metadata")
	}

	fileName := filepath.Join(h.basePath, agentFileName(meta))
	f, err := h.agentFileMap.Get(fileName)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf(
			"couldn't retrieve storage file: %s", fileName,
		)
	}
	h.agentFileMap.Set(fileName, f)

	if _, err := io.Copy(f, buf); err != nil {
		return http.StatusInternalServerError, fmt.Errorf(
			"failed writing to file: %s: %v", fileName, err,
		)
	}
	return http.StatusAccepted, nil
}

func (h requestHandler) processBatch(body io.ReadCloser, buf io.Writer, meta *metadata) error {
	byteBuf := h.bytesBufPool.Get().([]byte)
	defer func() {
		byteBuf = byteBuf[:0]
		h.bufPool.Put(byteBuf)
	}()
	defer body.Close()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(byteBuf, 0)
	var decodedMeta bool
	var err error
	for scanner.Scan() {
		line := scanner.Bytes()
		// Discard any lines that are at the maximum allowed length and aren't
		// completely scanned.
		if len(line) == maxScannerBufSize && line[len(line)-1] != '}' {
			continue
		}
		buf.Write(line)
		buf.Write([]byte("\n"))

		if decodedMeta {
			// continue scanning
			continue
		}
		if e := json.Unmarshal(scanner.Bytes(), &meta); e != nil {
			// TODO(marclop) multierror?
			err = e
			// Continue scanning, like we do in the APM Server itself.
			continue
		}
		// TODO(marclop) support RUM.
		decodedMeta = meta.V2.Service.Agent != nil
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return err
}

func agentFileName(meta metadata) string {
	const format = "%s-%s.ndjson"
	if meta.V2.Service.Agent != nil {
		agent := meta.V2.Service.Agent
		return fmt.Sprintf(format, agent.Name, agent.Version)
	}
	agent := meta.V3RUM.Service.Agent
	return fmt.Sprintf(format, agent.Name, agent.Version)
}

func logHandler(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		t := time.Now()
		h(rw, r)
		log.Printf("processed request to %s (%s) in %s\n",
			r.URL.Path,
			r.Header.Get("User-Agent"),
			time.Since(t).String(),
		)
	})
}

func (m *fileMap) Get(p string) (*syncFile, error) {
	if v, ok := m.m.Load(p); ok {
		return v.(*syncFile), nil
	}
	// NOTE(marclop) Optionally, the files could also be truncated.
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}
	return &syncFile{File: f}, nil
}

func (m *fileMap) Set(p string, f *syncFile) { m.m.Store(p, f) }

type syncFile struct {
	// We want to avoid multiple requests writing to the same file at the same
	// time, since may lead to incorrect events.
	mu sync.Mutex
	*os.File
}

func (f *syncFile) Write(b []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.File.Write(b)
}

func (f *syncFile) ReadFrom(r io.Reader) (n int64, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.File.ReadFrom(r)
}

type fileMap struct {
	m sync.Map
}

// Models

// Wraps both intake v2 and v2/rum metadata formats.
type metadata struct {
	V2    v2Metadata    `json:"metadata,omitempty"`
	V3RUM v3RUMMetadata `json:"m,omitempty"`
	// NOTE(marclop), we could do some decoding of the received data
	// to accumulate statistics.
}

func (m metadata) IsEmpty() bool {
	return (m.V2.Service.Agent == nil || m.V2.Service.Agent.Name == "") &&
		m.V3RUM.Service.Agent.Name == ""
}

type v2Metadata struct {
	Service model.Service `json:"service"`
}

type v3RUMMetadata struct {
	Service v3ServiceMeta `json:"se"`
}

type v3ServiceMeta struct {
	Agent   metadataServiceAgent `json:"a"`
	Name    string               `json:"n"`
	Version string               `json:"ve"`
}

type metadataServiceAgent struct {
	Name    string `json:"n"`
	Version string `json:"ve"`
}
