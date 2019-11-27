// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reporter

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const (
	// Whether to use grpc or tchannel reporter.
	reporterType = "reporter.type"
	// Agent tags
	agentTags = "jaeger.tags"
	// TCHANNEL is name of tchannel reporter.
	TCHANNEL Type = "tchannel"
	// GRPC is name of gRPC reporter.
	GRPC Type = "grpc"
)

// Type defines type of reporter.
type Type string

// Options holds generic reporter configuration.
type Options struct {
	ReporterType Type
	AgentTags    map[string]string
}

// AddFlags adds flags for Options.
func AddFlags(flags *flag.FlagSet) {
	flags.String(reporterType, string(GRPC), fmt.Sprintf("Reporter type to use e.g. %s, %s", string(GRPC), string(TCHANNEL)))
	flags.String(agentTags, "", "One or more tags to be added to the Process tags of all spans passing through this agent. Ex: key1=value1,key2=${envVar:defaultValue}")
}

// InitFromViper initializes Options with properties retrieved from Viper.
func (b *Options) InitFromViper(v *viper.Viper) *Options {
	b.ReporterType = Type(v.GetString(reporterType))
	b.AgentTags = parseAgentTags(v.GetString(agentTags))
	return b
}

// Parsing logic borrowed from jaegertracing/jaeger-client-go
func parseAgentTags(agentTags string) map[string]string {
	if agentTags == "" {
		return nil
	}
	tagPairs := strings.Split(string(agentTags), ",")
	tags := make(map[string]string)
	for _, p := range tagPairs {
		kv := strings.SplitN(p, "=", 2)
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])

		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			skipWhenEmpty := false

			ed := strings.SplitN(string(v[2:len(v)-1]), ":", 2)
			if len(ed) == 1 {
				// no default value specified, set to empty
				skipWhenEmpty = true
				ed = append(ed, "")
			}

			e, d := ed[0], ed[1]
			v = os.Getenv(e)
			if v == "" && d != "" {
				v = d
			}

			// no value is set, skip this entry
			if v == "" && skipWhenEmpty {
				continue
			}
		}

		tags[k] = v
	}

	return tags
}
