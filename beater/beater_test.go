package beater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/elastic/beats/libbeat/outputs"
	pubs "github.com/elastic/beats/libbeat/publisher"
	"github.com/elastic/beats/libbeat/publisher/pipeline"
	"github.com/elastic/beats/libbeat/publisher/queue"
	"github.com/elastic/beats/libbeat/publisher/queue/memqueue"
)

func TestBeatConfig(t *testing.T) {
	truthy := true
	tests := []struct {
		conf       map[string]interface{}
		beaterConf *Config
		SmapIndex  string
		msg        string
	}{
		{
			conf:       map[string]interface{}{},
			beaterConf: defaultConfig("6.2.0"),
			msg:        "Default config created for empty config.",
		},
		{
			conf: map[string]interface{}{
				"host":                   "localhost:3000",
				"max_unzipped_size":      64,
				"max_request_queue_time": 9 * time.Second,
				"max_header_size":        8,
				"read_timeout":           3 * time.Second,
				"write_timeout":          4 * time.Second,
				"shutdown_timeout":       9 * time.Second,
				"capture_personal_data":  true,
				"secret_token":           "1234random",
				"ssl": map[string]interface{}{
					"enabled":     true,
					"key":         "1234key",
					"certificate": "1234cert",
				},
				"concurrent_requests": 15,
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"frontend": map[string]interface{}{
					"enabled":       true,
					"rate_limit":    1000,
					"allow_origins": []string{"example*"},
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 5 * time.Minute,
						},
						"index_pattern": "apm-test*",
					},
					"library_pattern":       "^custom",
					"exclude_from_grouping": "^grouping",
				},
			},
			beaterConf: &Config{
				Host:                "localhost:3000",
				MaxUnzippedSize:     64,
				MaxRequestQueueTime: 9 * time.Second,
				MaxHeaderSize:       8,
				ReadTimeout:         3000000000,
				WriteTimeout:        4000000000,
				ShutdownTimeout:     9000000000,
				SecretToken:         "1234random",
				SSL:                 &SSLConfig{Enabled: &truthy, PrivateKey: "1234key", Cert: "1234cert"},
				AugmentEnabled:      true,
				Expvar: &ExpvarConfig{
					Enabled: &truthy,
					Url:     "/debug/vars",
				},
				Frontend: &FrontendConfig{
					Enabled:      &truthy,
					RateLimit:    1000,
					AllowOrigins: []string{"example*"},
					SourceMapping: &SourceMapping{
						Cache:        &Cache{Expiration: 5 * time.Minute},
						IndexPattern: "apm-test*",
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
					beatVersion:         "6.2.0",
				},
				ConcurrentRequests: 15,
			},
			msg: "Given config overwrites default",
		},
		{
			conf: map[string]interface{}{
				"host":              "localhost:3000",
				"max_unzipped_size": 64,
				"secret_token":      "1234random",
				"ssl": map[string]interface{}{
					"enabled": true,
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"frontend": map[string]interface{}{
					"enabled": true,
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 7,
						},
					},
				},
			},
			beaterConf: &Config{
				Host:                "localhost:3000",
				MaxUnzippedSize:     64,
				MaxRequestQueueTime: 2 * time.Second,
				MaxHeaderSize:       1048576,
				ReadTimeout:         2000000000,
				WriteTimeout:        2000000000,
				ShutdownTimeout:     5000000000,
				SecretToken:         "1234random",
				SSL:                 &SSLConfig{Enabled: &truthy, PrivateKey: "", Cert: ""},
				AugmentEnabled:      true,
				Expvar: &ExpvarConfig{
					Enabled: &truthy,
					Url:     "/debug/vars",
				},
				Frontend: &FrontendConfig{
					Enabled:      &truthy,
					RateLimit:    10,
					AllowOrigins: []string{"*"},
					SourceMapping: &SourceMapping{
						Cache: &Cache{
							Expiration: 7 * time.Second,
						},
						IndexPattern: "apm-*-sourcemap*",
					},
					LibraryPattern:      "node_modules|bower_components|~",
					ExcludeFromGrouping: "^/webpack",
					beatVersion:         "6.2.0",
				},
				ConcurrentRequests: 5,
			},
			msg: "Given config merged with default",
		},
	}

	for _, test := range tests {
		ucfgConfig, err := common.NewConfigFrom(test.conf)
		assert.NoError(t, err)
		btr, err := New(&beat.Beat{Info: beat.Info{Version: "6.2.0"}}, ucfgConfig)
		assert.NoError(t, err)
		assert.NotNil(t, btr)
		bt := btr.(*beater)
		assert.Equal(t, test.beaterConf, bt.config, test.msg)
	}
}

/*
Run the benchmarks as follows:

	$ go test beater/*.go -run=XXX -bench=. -cpuprofile=cpu.out

then load the cpu profile file:

	$ go tool pprof beater.test cpu.out

type `web` to get a nice svg that shows the call graph and time spent:

	(pprof) web

To get a memory profile, use this:

	$ go test beater/*.go -run=XXX -bench=. -memprofile=mem.out

*/

// Needed to make unique registers
// TODO: When pipeline no longer requires a *monitoring.Registry,
//       this can be removed.
var testCount int

type DummyOutputClient struct {
}

func (d *DummyOutputClient) Publish(batch pubs.Batch) error {
	batch.ACK()
	return nil
}

func (d *DummyOutputClient) Close() error {
	return nil
}

func SetupServer(b *testing.B) *http.ServeMux {
	out := outputs.Group{
		Clients:   []outputs.Client{&DummyOutputClient{}},
		BatchSize: 5,
		Retry:     0, // no retry. on error drop events
	}

	queueFactory := func(e queue.Eventer) (queue.Queue, error) {
		return memqueue.NewBroker(memqueue.Settings{
			Eventer: e,
			Events:  20,
		}), nil
	}
	testCount++
	pip, err := pipeline.New(
		beat.Info{Name: "testBeat"},
		monitoring.Default.NewRegistry("testing"+string(testCount)),
		queueFactory, out, pipeline.Settings{
			WaitClose:     0,
			WaitCloseMode: pipeline.NoWaitOnClose,
		})

	if err != nil {
		b.Fatalf("error initializing publisher: %v", err)
	}

	pub, err := newPublisher(pip, 1)

	if err != nil {
		b.Fatal(err)
	}
	return newMuxer(defaultConfig("7.0.0"), pub.Send)
}

func pluralize(entity string) string {
	return entity + "s"
}
func createPayload(entityType string, numEntities int) []byte {
	data, err := loader.LoadValidData(entityType)
	if err != nil {
		panic(err)
	}
	var entityList []interface{}
	testEntities := data[pluralize(entityType)].([]interface{})

	for i := 0; i < numEntities; i++ {
		entityList = append(entityList, testEntities[i%len(testEntities)])
	}
	data[pluralize(entityType)] = entityList
	out, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return out
}

func benchmarkVariableSizePayload(b *testing.B, entitytype string, entries int) {
	url := "/v1/" + pluralize(entitytype)
	mux := SetupServer(b)
	data := createPayload(entitytype, entries)
	b.Logf("Using payload size: %d", len(data))
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		req.Header.Add("Content-Type", "application/json")
		if err != nil {
			b.Error(err)
		}

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 202 {
			b.Fatal(w.Body.String())
		}
	}
}

func BenchmarkServer(b *testing.B) {
	entityTypes := []string{"transaction", "error"}
	sizes := []int{100, 1000, 10000}

	for _, et := range entityTypes {
		b.Run(et, func(b *testing.B) {
			for _, sz := range sizes {
				b.Run(fmt.Sprintf("size=%v", sz), func(b *testing.B) {
					benchmarkVariableSizePayload(b, et, sz)
				})
			}
		})
	}
}
