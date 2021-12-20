package main

import (
	"flag"
	"fmt"
	"math"
	"sync"
	"time"

	"go.elastic.co/apm"
)

func init() {
	apm.DefaultTracer.Close()
}

func main() {
	var (
		cooldown time.Duration
		çount    uint
	)
	flag.UintVar(&çount, "count", 30, "Number of times to run the load generator")
	flag.DurationVar(&cooldown, "cooldown", 2*time.Millisecond, "time to wait between transactions / spans")
	flag.Parse()

	t := time.Now()
	for i := 0; i < int(çount); i++ {
		multipleTransactions(int(çount), cooldown)
	}

	fmt.Println("Ran for", time.Now().Sub(t).String())
}

func multipleTransactions(count int, cooldown time.Duration) {
	var wg sync.WaitGroup
	for j := 0; j < count; j++ {
		wg.Add(1)
		go func() {
			tracer, err := apm.NewTracer("goagent-different", "1.0.0")
			if err != nil {
				panic(err)
			}
			for i := 0; i < 1000; i++ {
				time.Sleep(cooldown)
				withSpan(tracer)
				if math.Mod(float64(i), 100) == 0 {
					tracer.Flush(nil)
				}
			}
			wg.Done()
			tracer.SendMetrics(nil)
			fmt.Println(fmt.Sprintf("%+v", tracer.Stats()))
			tracer.Close()
		}()
	}
	spanRunners := 2 * count
	for j := 0; j < spanRunners; j++ {
		wg.Add(1)
		go func() {
			tracer, err := apm.NewTracer("goagent", "1.0.0")
			if err != nil {
				panic(err)
			}
			for i := 0; i < 10000; i++ {
				time.Sleep(cooldown)
				withSpan(tracer)
				if math.Mod(float64(i), 100) == 0 {
					tracer.Flush(nil)
				}
			}
			wg.Done()
			tracer.SendMetrics(nil)
			fmt.Println(fmt.Sprintf("%+v", tracer.Stats()))
			tracer.Close()
		}()
	}
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
