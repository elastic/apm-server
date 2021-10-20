package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"go.elastic.co/apm"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		tracer, err := apm.NewTracer("goagent", "1.0.0")
		if err != nil {
			panic(err)
		}
		for i := 0; i < 1000; i++ {
			withTransaction(tracer)
			if math.Mod(float64(i), 50) == 0 {
				tracer.Flush(nil)
			}
		}
		time.Sleep(10 * time.Second)
		wg.Done()
		fmt.Println(fmt.Sprintf("%+v", tracer.Stats()))
	}()
	wg.Add(1)
	go func() {
		tracer, err := apm.NewTracer("goagent", "1.0.0")
		if err != nil {
			panic(err)
		}
		for i := 0; i < 1000; i++ {
			withSpan(tracer)
			if math.Mod(float64(i), 50) == 0 {
				tracer.Flush(nil)
			}
		}
		time.Sleep(10 * time.Second)
		wg.Done()
		fmt.Println(fmt.Sprintf("%+v", tracer.Stats()))
	}()
	wg.Wait()
}

func withTransaction(tracer *apm.Tracer) {
	tx := tracer.StartTransaction("no_spans", "request")
	defer tx.End()
	time.Sleep(5 * time.Millisecond)
	tx.Result = "HTTP 2xx"
}

func withSpan(tracer *apm.Tracer) {
	tx := tracer.StartTransaction("with_spans", "request")
	defer tx.End()
	span := tx.StartSpan("SELECT FROM foo", "db.mysql.query", nil)
	defer span.End()
	time.Sleep(5 * time.Millisecond)
	tx.Result = "HTTP 2xx"
}
