package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"google.golang.org/grpc"
)

var (
	serverAddr = flag.String("addr", "localhost:14250", "Jaeger gRPC server address")
	insecure   = flag.Bool("insecure", false, "Disable certificate verification")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
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

func decodeRequest(filename string) (*api_v2.PostSpansRequest, error) {
	var request api_v2.PostSpansRequest
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return &request, json.NewDecoder(f).Decode(&request)
}
