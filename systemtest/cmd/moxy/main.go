package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logLevel := zap.LevelFlag(
		"loglevel", zapcore.InfoLevel,
		"set log level to one of: DEBUG, INFO (default), WARN, ERROR, DPANIC, PANIC, FATAL",
	)
	username := flag.String("username", "elastic", "authentication username to mimic ES")
	password := flag.String("password", "", "authentication username to mimic ES")
	flag.Parse()
	zapcfg := zap.NewProductionConfig()
	zapcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapcfg.Encoding = "console"
	zapcfg.Level = zap.NewAtomicLevelAt(*logLevel)
	logger, err := zapcfg.Build()
	if err != nil {
		panic(err)
	}
	if *username == "" || *password == "" {
		logger.Fatal("both username and password are required")
	}
	defer logger.Sync()
	s := http.Server{
		Addr:    ":9200",
		Handler: handler(logger, *username, *password),
	}
	if err := s.ListenAndServe(); err != nil {
		logger.Fatal("listen error", zap.Error(err))
	}
}

func handler(logger *zap.Logger, username, password string) http.Handler {
	expectedAuth := fmt.Sprintf("%s:%s", username, password)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		auth := r.Header.Get("Authorization")
		actualAuth, err := base64.StdEncoding.DecodeString(auth)
		if err != nil || string(actualAuth) != expectedAuth {
			logger.Error(
				"authentication failed",
				zap.Error(err),
				zap.String("actual", string(actualAuth)),
				zap.String("expected", expectedAuth),
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/":
			// MIS doesn't use this route, but apm-server checks for cluster_uuid
			w.Write([]byte(`{
			"name": "instance-0000000001",
			"cluster_name": "eca3b3c3bbee4816bb92f82184e328dd",
			"cluster_uuid": "cc49813b6b8e2138fbb8243ae2b3deed",
			"version": {
				"number": "8.15.1",
				"build_flavor": "default",
				"build_type": "docker",
				"build_hash": "253e8544a65ad44581194068936f2a5d57c2c051",
				"build_date": "2024-09-02T22:04:47.310170297Z",
				"build_snapshot": false,
				"lucene_version": "9.11.1",
				"minimum_wire_compatibility_version": "7.17.0",
				"minimum_index_compatibility_version": "7.0.0"
			},
			"tagline": "You Know, for Search"
			}`))
		case "/_security/user/_has_privileges":
			w.Write([]byte(`{"username":"admin","has_all_requested":true,"cluster":{},"index":{},"application":{"apm":{"-":{"event:write":true}}}}`))
		case "/_bulk":
			first := true
			var body io.Reader
			switch r.Header.Get("Content-Encoding") {
			case "gzip":
				r, err := gzip.NewReader(r.Body)
				if err != nil {
					logger.Error("gzip reader err", zap.Error(err))
					http.Error(w, fmt.Sprintf("reader error: %v", err), http.StatusInternalServerError)
					return
				}
				defer r.Close()
				body = r
			case "zstd":
				r, err := zstd.NewReader(r.Body)
				if err != nil {
					logger.Error("zstd reader err", zap.Error(err))
					http.Error(w, fmt.Sprintf("reader error: %v", err), http.StatusInternalServerError)
					return
				}
				defer r.Close()
				body = r
			default:
				body = r.Body
			}

			var jsonw bytes.Buffer
			jsonw.Write([]byte(`{"items":[`))
			scanner := bufio.NewScanner(body)
			for scanner.Scan() {
				// Action is always "create", skip decoding.
				if !scanner.Scan() {
					logger.Error("unexpected payload")
					http.Error(w, "expected source", http.StatusInternalServerError)
					return
				}
				if first {
					first = false
				} else {
					jsonw.WriteByte(',')
				}
				jsonw.Write([]byte(`{"create":{"status":201}}`))
			}
			if err := scanner.Err(); err != nil {
				logger.Error("scanner error", zap.Error(err))
				http.Error(w, fmt.Sprintf("scanner error: %v", err), http.StatusInternalServerError)
			} else {
				jsonw.Write([]byte(`]}`))
				w.Write(jsonw.Bytes())
			}
		default:
			logger.Error("unknown path", zap.String("path", r.URL.Path))
		}
	})
}
