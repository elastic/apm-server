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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var (
	corporaDir string
	repoRoot   string
)

func init() {
	// Locate the "cat_bulk.py" script, which runs apm-server and
	// writes Elasticsearch docs to stdout. The script is separated
	// from gencorpora code so it can be reused.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("failed to locate source file")
	}
	gencorporaDir := filepath.Dir(file)
	corporaDir = filepath.Join(gencorporaDir, "../corpora")
	repoRoot = filepath.Join(gencorporaDir, "../../")
}

type newTracerFunc func(serviceName string) trace.Tracer

// GenerateCorpus generates a Rally document corpus with the given name,
// by ingesting events from the given scenario function through APM Server.
//
// The corpus will be written corpora/<name>.json (metadata), and
// corpora/<name>_documents.json (documents).
func GenerateCorpus(name string, scenario func(newTracerFunc)) error {
	log.Printf("Generating corpus %q", name)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var output []byte
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		output, err = runServer(ctx)
		return err
	})

	// Wait for server to be running.
	serverURL := &url.URL{Scheme: "http", Host: "127.0.0.1:8200"}
	if err := waitRunning(ctx, serverURL.String()); err != nil {
		return err
	}
	conn, err := grpc.DialContext(ctx, serverURL.Host, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return err
	}
	spanProcessor := sdktrace.NewSimpleSpanProcessor(traceExporter)
	defer spanProcessor.Shutdown(context.Background())

	newTracer := func(serviceName string) trace.Tracer {
		res, err := resource.New(context.Background(),
			resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
		)
		if err != nil {
			log.Fatal(err)
		}
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
			sdktrace.WithSpanProcessor(spanProcessor),
		)
		return tracerProvider.Tracer("")
	}
	scenario(newTracer)

	cancel()
	if err := g.Wait(); err != nil {
		return err
	}
	return writeCorpus(name, output)
}

func writeCorpus(name string, documents []byte) error {
	documentsFilename := name + "_documents.json"

	var metadata struct {
		SourceFile                  string `json:"source-file"`
		DocumentCount               int    `json:"document-count"`
		UncompressedBytes           int    `json:"uncompressed-bytes"`
		IncludedsActionAndMetaAdata bool   `json:"includes-action-and-meta-data"`
	}
	metadata.SourceFile = path.Join("corpora", documentsFilename)
	metadata.DocumentCount = bytes.Count(documents, []byte("\n")) / 2
	metadata.UncompressedBytes = len(documents)
	metadata.IncludedsActionAndMetaAdata = true

	metadataPath := filepath.Join(corporaDir, name+".json")
	documentsPath := filepath.Join(corporaDir, documentsFilename)
	if err := ioutil.WriteFile(documentsPath, documents, 0644); err != nil {
		return err
	}
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(metadataPath, append(metadataBytes, '\n'), 0644); err != nil {
		return err
	}

	relDocumentsPath, _ := filepath.Rel(repoRoot, documentsPath)
	relMetadataPath, _ := filepath.Rel(repoRoot, metadataPath)

	log.Printf("- wrote %d documents to %s", metadata.DocumentCount, relDocumentsPath)
	log.Printf("- wrote metadata to %s", relMetadataPath)
	return nil
}

func waitRunning(ctx context.Context, serverURL string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		resp, err := http.Get(serverURL)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}
}

func runServer(ctx context.Context) ([]byte, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(filepath.Join(repoRoot, "script", "cat_bulk.py"))
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	<-ctx.Done()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return stdout.Bytes(), nil
}
