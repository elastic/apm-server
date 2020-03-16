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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"google.golang.org/grpc"
)

var (
	serverAddr = flag.String("addr", "localhost:14250", "Jaeger gRPC server address")
	insecure   = flag.Bool("insecure", false, "Disable certificate verification")
	endpoint   = flag.String("endpoint", "collector", "Which Jaeger gRPC endpoint to call ('collector', 'sampler')")
	service    = flag.String("service", "xyz", "Service for which sampling rate should be fetched")
	path       = flag.String("out", "", "Output path for sampling response")
)

func main() {
	flag.Parse()
	if *endpoint != "sampler" && flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <request.json> [<request2.json> ...]\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	var opts []grpc.DialOption
	if *insecure {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	switch *endpoint {
	case "sampler":
		if *path == "" {
			log.Fatal("output path missing")
		}
		p, err := filepath.Abs(*path)
		if err != nil {
			log.Fatal(err)
		}
		os.Remove(p)
		client := api_v2.NewSamplingManagerClient(conn)
		resp, err := client.GetSamplingStrategy(context.Background(), &api_v2.SamplingStrategyParameters{ServiceName: *service})
		if err != nil {
			log.Fatal(err)
		}
		s := fmt.Sprintf("strategy: %s, sampling rate: %v", resp.StrategyType.String(), resp.ProbabilisticSampling.SamplingRate)
		if err := ioutil.WriteFile(p, []byte(s), 0644); err != nil {
			log.Fatal(err)
		}

	default:
		client := api_v2.NewCollectorServiceClient(conn)
		for _, arg := range flag.Args() {
			request, err := decodeRequest(arg)
			if err != nil {
				log.Fatal(err)
			}
			_, err = client.PostSpans(context.Background(), request)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func decodeRequest(filename string) (*api_v2.PostSpansRequest, error) {
	var request api_v2.PostSpansRequest
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return &request, json.NewDecoder(f).Decode(&request)
}
