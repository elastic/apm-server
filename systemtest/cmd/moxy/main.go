package main

import (
	"bufio"
	"bytes"
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
	defer logger.Sync()
	s := http.Server{
		Addr:    ":9200",
		Handler: handler(logger),
	}
	if err := s.ListenAndServe(); err != nil {
		logger.Fatal("listen error", zap.Error(err))
	}
}

func handler(logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		first := true
		switch r.URL.Path {
		case "/_security/user/_has_privileges":
			w.Write([]byte(`{"username":"admin","has_all_requested":true,"cluster":{},"index":{},"application":{"apm":{"-":{"event:write":true}}}}`))
		case "/_bulk":
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
