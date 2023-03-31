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

package loadgencfg

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var Config struct {
	ServerURL         *url.URL
	SecretToken       string
	APIKey            string
	Secure            bool
	EventRate         RateFlag
	RewriteIDs        bool
	RewriteTimestamps bool
	Headers           map[string]string
}

type RateFlag struct {
	Burst    int
	Interval time.Duration
}

func (f *RateFlag) String() string {
	return fmt.Sprintf("%d/%s", f.Burst, f.Interval)
}

func (f *RateFlag) Set(s string) error {
	before, after, ok := strings.Cut(s, "/")
	if !ok || before == "" || after == "" {
		return fmt.Errorf("invalid rate %q, expected format burst/duration", s)
	}

	burst, err := strconv.Atoi(before)
	if err != nil {
		return fmt.Errorf("invalid burst %s in event rate: %w", before, err)
	}

	if !(after[0] >= '0' && after[0] <= '9') {
		after = "1" + after
	}
	interval, err := time.ParseDuration(after)
	if err != nil {
		return fmt.Errorf("invalid interval %q in event rate: %w", after, err)
	}
	if interval <= 0 {
		return fmt.Errorf("invalid interval %q, must be positive", after)
	}

	f.Burst = burst
	f.Interval = interval
	return nil
}

func init() {
	// Server config
	flag.Func(
		"server",
		"apm-server URL (default http://127.0.0.1:8200)",
		func(server string) (err error) {
			if server != "" {
				Config.ServerURL, err = url.Parse(server)
			}
			return
		})
	flag.StringVar(&Config.SecretToken, "secret-token", "", "secret token for APM Server")
	flag.StringVar(&Config.APIKey, "api-key", "", "API key for APM Server")
	flag.BoolVar(&Config.Secure, "secure", false, "validate the remote server TLS certificates")
	flag.BoolVar(
		&Config.RewriteTimestamps,
		"rewrite-timestamps",
		false,
		"rewrite event timestamps every iteration, maintaining relative offsets",
	)

	flag.BoolVar(
		&Config.RewriteIDs,
		"rewrite-ids",
		false,
		"rewrite event IDs every iteration, maintaining event relationships",
	)
	flag.Func("header",
		"extra headers to use when sending data to the apm-server",
		func(s string) error {
			k, v, ok := strings.Cut(s, "=")
			if !ok {
				return fmt.Errorf("invalid header '%s': format must be key=value", s)
			}
			if len(Config.Headers) == 0 {
				Config.Headers = make(map[string]string)
			}
			Config.Headers[k] = v
			return nil
		},
	)
	flag.Var(&Config.EventRate, "event-rate", "Event rate in format of {burst}/{interval}. For example, 200/5s, <= 0 values evaluate to Inf (default 0/s)")

	// For configs that can be set via environment variables, set the required
	// flags from env if they are not explicitly provided via command line
	setFlagsFromEnv()
}

func setFlagsFromEnv() {
	// value[0] is environment key
	// value[1] is default value
	flagEnvMap := map[string][]string{
		"server":       {"ELASTIC_APM_SERVER_URL", "http://127.0.0.1:8200"},
		"secret-token": {"ELASTIC_APM_SECRET_TOKEN", ""},
		"api-key":      {"ELASTIC_APM_API_KEY", ""},
		"secure":       {"ELASTIC_APM_VERIFY_SERVER_CERT", "false"},
	}

	for k, v := range flagEnvMap {
		flag.Set(k, getEnvOrDefault(v[0], v[1]))
	}
}

func getEnvOrDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return defaultValue
}
