// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
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

package app

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/spf13/viper"

	"github.com/jaegertracing/jaeger/ports"
)

const (
	suffixWorkers             = "workers"
	suffixServerQueueSize     = "server-queue-size"
	suffixServerMaxPacketSize = "server-max-packet-size"
	suffixServerHostPort      = "server-host-port"
	httpServerHostPort        = "http-server.host-port"
)

var defaultProcessors = []struct {
	model    Model
	protocol Protocol
	port     int
}{
	{model: "zipkin", protocol: "compact", port: ports.AgentZipkinThriftCompactUDP},
	{model: "jaeger", protocol: "compact", port: ports.AgentJaegerThriftCompactUDP},
	{model: "jaeger", protocol: "binary", port: ports.AgentJaegerThriftBinaryUDP},
}

// AddFlags adds flags for Builder.
func AddFlags(flags *flag.FlagSet) {
	for _, p := range defaultProcessors {
		prefix := fmt.Sprintf("processor.%s-%s.", p.model, p.protocol)
		flags.Int(prefix+suffixWorkers, defaultServerWorkers, "how many workers the processor should run")
		flags.Int(prefix+suffixServerQueueSize, defaultQueueSize, "length of the queue for the UDP server")
		flags.Int(prefix+suffixServerMaxPacketSize, defaultMaxPacketSize, "max packet size for the UDP server")
		flags.String(prefix+suffixServerHostPort, ":"+strconv.Itoa(p.port), "host:port for the UDP server")
	}
	flags.String(
		httpServerHostPort,
		defaultHTTPServerHostPort,
		"host:port of the http server (e.g. for /sampling point and /baggageRestrictions endpoint)")
}

// InitFromViper initializes Builder with properties retrieved from Viper.
func (b *Builder) InitFromViper(v *viper.Viper) *Builder {
	for _, processor := range defaultProcessors {
		prefix := fmt.Sprintf("processor.%s-%s.", processor.model, processor.protocol)
		p := &ProcessorConfiguration{Model: processor.model, Protocol: processor.protocol}
		p.Workers = v.GetInt(prefix + suffixWorkers)
		p.Server.QueueSize = v.GetInt(prefix + suffixServerQueueSize)
		p.Server.MaxPacketSize = v.GetInt(prefix + suffixServerMaxPacketSize)
		p.Server.HostPort = v.GetString(prefix + suffixServerHostPort)
		b.Processors = append(b.Processors, *p)
	}

	b.HTTPServer.HostPort = v.GetString(httpServerHostPort)
	return b
}
