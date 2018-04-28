package elasticapm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/transport"
)

// ExampleTracer shows how to use the Tracer API
func ExampleTracer() {
	var r recorder
	server := httptest.NewServer(&r)
	defer server.Close()

	// ELASTIC_APM_SERVER_URL should typically set in the environment
	// when the process is started. The InitDefault call below is only
	// required in this case because the environment variable is set
	// after the program has been initialized.
	os.Setenv("ELASTIC_APM_SERVER_URL", server.URL)
	defer os.Unsetenv("ELASTIC_APM_SERVER_URL")
	transport.InitDefault()

	const serviceName = "service-name"
	const serviceVersion = "1.0.0"
	tracer, err := elasticapm.NewTracer(serviceName, serviceVersion)
	if err != nil {
		log.Fatal(err)
	}
	defer tracer.Close()

	// api is a very basic API handler, to demonstrate the usage
	// of the tracer. api.handlerOrder creates a transaction for
	// every call; api.handleOrder calls through to storeOrder,
	// which adds a span to the transaction.
	api := &api{tracer: tracer}
	api.handleOrder(context.Background(), "fish fingers")
	api.handleOrder(context.Background(), "detergent")

	// The tracer will queue transactions to be sent at a later time,
	// or when the queue becomes full. The interval and queue size
	// can both be configured by setting the environment variables
	// ELASTIC_APM_FLUSH_INTERVAL and ELASTIC_APM_MAX_QUEUE_SIZE
	// respectively. Alternatively, the tracer's SetFlushInterval
	// and SetMaxQueueSize methods can be used.
	tracer.Flush(nil)

	fmt.Println("number of payloads:", len(r.payloads))
	p0 := r.payloads[0]
	service := p0["service"].(map[string]interface{})
	agent := service["agent"].(map[string]interface{})
	language := service["language"].(map[string]interface{})
	runtime := service["runtime"].(map[string]interface{})
	transactions := p0["transactions"].([]interface{})
	fmt.Println("  service name:", service["name"])
	fmt.Println("  service version:", service["version"])
	fmt.Println("  agent name:", agent["name"])
	fmt.Println("  language name:", language["name"])
	fmt.Println("  runtime name:", runtime["name"])
	fmt.Println("  number of transactions:", len(transactions))
	for i := range transactions {
		t := transactions[i].(map[string]interface{})
		fmt.Println("    transaction name:", t["name"])
		fmt.Println("    transaction type:", t["type"])
		fmt.Println("    transaction context:", t["context"])
		spans := t["spans"].([]interface{})
		fmt.Println("    number of spans:", len(spans))
		s0 := spans[0].(map[string]interface{})
		fmt.Println("      span name:", s0["name"])
		fmt.Println("      span type:", s0["type"])
	}

	// Output:
	// number of payloads: 1
	//   service name: service-name
	//   service version: 1.0.0
	//   agent name: go
	//   language name: go
	//   runtime name: gc
	//   number of transactions: 2
	//     transaction name: order
	//     transaction type: request
	//     transaction context: map[custom:map[product:fish fingers]]
	//     number of spans: 1
	//       span name: store_order
	//       span type: rpc
	//     transaction name: order
	//     transaction type: request
	//     transaction context: map[custom:map[product:detergent]]
	//     number of spans: 1
	//       span name: store_order
	//       span type: rpc
}

type api struct {
	tracer *elasticapm.Tracer
}

func (api *api) handleOrder(ctx context.Context, product string) {
	tx := api.tracer.StartTransaction("order", "request")
	defer tx.Done(-1)
	ctx = elasticapm.ContextWithTransaction(ctx, tx)

	tx.Context.SetCustom("product", product)

	time.Sleep(10 * time.Millisecond)
	storeOrder(ctx, product)
	time.Sleep(20 * time.Millisecond)
}

func storeOrder(ctx context.Context, product string) {
	span, _ := elasticapm.StartSpan(ctx, "store_order", "rpc")
	if span != nil {
		defer span.Done(-1)
	}

	time.Sleep(50 * time.Millisecond)
}

type recorder struct {
	mu       sync.Mutex
	payloads []map[string]interface{}
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.payloads)
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var m map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	r.mu.Lock()
	r.payloads = append(r.payloads, m)
	r.mu.Unlock()
}
