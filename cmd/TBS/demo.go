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
