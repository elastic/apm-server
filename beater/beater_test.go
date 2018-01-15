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

	"github.com/elastic/apm-server/tests"
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
		msg        string
	}{
		{
			conf:       map[string]interface{}{},
			beaterConf: defaultConfig(),
			msg:        "Default config created for empty config.",
		},
		{
			conf: map[string]interface{}{
				"host":              "localhost:3000",
				"max_unzipped_size": 64,
				"max_header_size":   8,
				"read_timeout":      3 * time.Second,
				"write_timeout":     4 * time.Second,
				"shutdown_timeout":  9 * time.Second,
				"secret_token":      "1234random",
				"ssl": map[string]interface{}{
					"enabled":     true,
					"key":         "1234key",
					"certificate": "1234cert",
				},
				"concurrent_requests": 15,
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
					"library_pattern": "^custom",
				},
			},
			beaterConf: &Config{
				Host:            "localhost:3000",
				MaxUnzippedSize: 64,
				MaxHeaderSize:   8,
				ReadTimeout:     3000000000,
				WriteTimeout:    4000000000,
				ShutdownTimeout: 9000000000,
				SecretToken:     "1234random",
				SSL:             &SSLConfig{Enabled: &truthy, PrivateKey: "1234key", Cert: "1234cert"},
				Frontend: &FrontendConfig{
					Enabled:      &truthy,
					RateLimit:    1000,
					AllowOrigins: []string{"example*"},
					SourceMapping: &SourceMapping{
						Cache: &Cache{Expiration: 5 * time.Minute},
						Index: "apm-test*",
					},
					LibraryPattern: "^custom",
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
				Host:            "localhost:3000",
				MaxUnzippedSize: 64,
				MaxHeaderSize:   1048576,
				ReadTimeout:     2000000000,
				WriteTimeout:    2000000000,
				ShutdownTimeout: 5000000000,
				SecretToken:     "1234random",
				SSL:             &SSLConfig{Enabled: &truthy, PrivateKey: "", Cert: ""},
				Frontend: &FrontendConfig{
					Enabled:      &truthy,
					RateLimit:    10,
					AllowOrigins: []string{"*"},
					SourceMapping: &SourceMapping{
						Cache: &Cache{
							Expiration: 7 * time.Second,
						},
						Index: "apm",
					},
					LibraryPattern: "node_modules|bower_components|~",
				},
				ConcurrentRequests: 40,
			},
			msg: "Given config merged with default",
		},
	}

	for _, test := range tests {
		ucfgConfig, err := common.NewConfigFrom(test.conf)
		assert.NoError(t, err)
		btr, err := New(&beat.Beat{}, ucfgConfig)
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
	return newMuxer(defaultConfig(), pub.Send)
}

func pluralize(entity string) string {
	return entity + "s"
}
func createPayload(entitytype string, numEntities int) []byte {
	data, err := tests.LoadValidData(entitytype)
	if err != nil {
		panic(err)
	}
	var entityList []interface{}
	testEntities := data[pluralize(entitytype)].([]interface{})

	for i := 0; i < numEntities; i++ {
		entityList = append(entityList, testEntities[i%len(testEntities)])
	}
	data[pluralize(entitytype)] = entityList
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
