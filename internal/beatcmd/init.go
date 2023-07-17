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

package beatcmd

import (
	cryptorand "crypto/rand"
	"log"
	"math"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/cfgfile"
	_ "github.com/elastic/beats/v7/libbeat/monitoring/report/elasticsearch" // register default monitoring reporting
	_ "github.com/elastic/beats/v7/libbeat/outputs/codec/json"
	_ "github.com/elastic/beats/v7/libbeat/outputs/console"
	_ "github.com/elastic/beats/v7/libbeat/outputs/fileout"
	_ "github.com/elastic/beats/v7/libbeat/outputs/kafka"
	_ "github.com/elastic/beats/v7/libbeat/outputs/logstash"
	_ "github.com/elastic/beats/v7/libbeat/outputs/redis"
	_ "github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"
)

func init() {
	initRand()
	initFlags()
}

func initRand() {
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(math.MaxInt64))
	var seed int64
	if err != nil {
		seed = time.Now().UnixNano()
	} else {
		seed = n.Int64()
	}
	rand.Seed(seed) //lint:ignore SA1019 libbeat uses deprecated math/rand functions prolifically
}

func initFlags() {
	// For backwards compatibility, convert -flags to --flags.
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			os.Args[1+i] = "-" + arg
		}
	}

	if err := cfgfile.HandleFlags(); err != nil {
		log.Fatal(err)
	}
}
