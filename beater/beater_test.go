package beater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	// make sure processors are loaded
	_ "github.com/elastic/apm-server/include"

	"github.com/elastic/apm-server/tests"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/elastic/beats/libbeat/outputs"
	pubs "github.com/elastic/beats/libbeat/publisher"
	"github.com/elastic/beats/libbeat/publisher/pipeline"
	"github.com/elastic/beats/libbeat/publisher/queue"
	"github.com/elastic/beats/libbeat/publisher/queue/memqueue"
)

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
	return newMuxer(defaultConfig, pub.Send)
}

func pluralize(entity string) string {
	return entity + "s"
}
func createPayload(entitytype string, numEntities int) []byte {
	data, err := tests.LoadValidData(entitytype)
	if err != nil {
		panic(err)
	}
	var payload map[string]interface{}
	err = json.Unmarshal(data, &payload)
	var entityList []interface{}
	testEntities := payload[pluralize(entitytype)].([]interface{})

	for i := 0; i < numEntities; i++ {
		entityList = append(entityList, testEntities[i%len(testEntities)])
	}
	payload[pluralize(entitytype)] = entityList
	out, err := json.Marshal(payload)
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
