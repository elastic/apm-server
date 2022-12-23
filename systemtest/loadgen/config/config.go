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
)

var Config struct {
	ServerURL   *url.URL
	SecretToken string
	Secure      bool
	MaxEPM      int
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
	flag.BoolVar(&Config.Secure, "secure", false, "validate the remote server TLS certificates")
	flag.Func(
		"max-rate",
		"Max event rate as epm or eps with burst size=max(1000, 2*eps), <= 0 values evaluate to Inf (default 0epm)",
		func(rate string) error {
			errStr := "invalid value %s for -max-rate, valid examples: 5eps or 10epm"
			r := strings.Split(rate, "ep")
			if len(r) != 2 {
				return fmt.Errorf(errStr, rate)
			}
			rateVal, err := strconv.Atoi(r[0])
			if err != nil {
				return fmt.Errorf(errStr, rate)
			}
			switch r[1] {
			case "s":
				Config.MaxEPM = rateVal * 60
			case "m":
				Config.MaxEPM = rateVal
			default:
				return fmt.Errorf(errStr, rate)
			}
			return nil
		})

	// For configs that can be set via environment variables, set the required
	// flags from env if they are not explicitly provided via command line
	setFlagsFromEnv()
}

func setFlagsFromEnv() {
	// value[0] is environment key
	// value[1] is default value
	flagEnvMap := map[string][]string{
		"server":       []string{"ELASTIC_APM_SERVER_URL", "http://127.0.0.1:8200"},
		"secret-token": []string{"ELASTIC_APM_SECRET_TOKEN", ""},
		"secure":       []string{"ELASTIC_APM_VERIFY_SERVER_CERT", "false"},
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
