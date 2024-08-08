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

package modelprocessor

// IsInternalMetricName returns true when the metric is considered "internal",
// and is stored in a strictly mapped data stream. Updates to the language /
// runtime mappings on the internal_metrics APM package must be kept up to date
// with the fields defined here.
func IsInternalMetricName(name string) bool {
	switch name {
	case "agent.background.cpu.overhead.pct":
		return true
	case "agent.background.cpu.total.pct":
		return true
	case "agent.background.memory.allocation.bytes":
		return true
	case "agent.events.dropped":
		return true
	case "agent.events.queue.max_size.pct":
		return true
	case "agent.events.queue.min_size.pct":
		return true
	case "agent.events.requests.bytes":
		return true
	case "agent.events.requests.count":
		return true
	case "agent.events.total":
		return true
	case "clr.gc.count":
		return true
	case "clr.gc.gen0size":
		return true
	case "clr.gc.gen1size":
		return true
	case "clr.gc.gen2size":
		return true
	case "clr.gc.gen3size":
		return true
	case "clr.gc.time":
		return true
	case "faas.billed_duration":
		return true
	case "faas.coldstart_duration":
		return true
	case "faas.duration":
		return true
	case "faas.timeout":
		return true
	case "golang.goroutines":
		return true
	case "golang.heap.allocations.active":
		return true
	case "golang.heap.allocations.allocated":
		return true
	case "golang.heap.allocations.frees":
		return true
	case "golang.heap.allocations.idle":
		return true
	case "golang.heap.allocations.mallocs":
		return true
	case "golang.heap.allocations.objects":
		return true
	case "golang.heap.allocations.total":
		return true
	case "golang.heap.gc.cpu_fraction":
		return true
	case "golang.heap.gc.next_gc_limit":
		return true
	case "golang.heap.gc.total_count":
		return true
	case "golang.heap.gc.total_pause.ns":
		return true
	case "golang.heap.system.obtained":
		return true
	case "golang.heap.system.released":
		return true
	case "golang.heap.system.stack":
		return true
	case "golang.heap.system.total":
		return true
	case "jvm.gc.alloc":
		return true
	case "jvm.gc.count":
		return true
	case "jvm.gc.time":
		return true
	case "jvm.memory.heap.committed":
		return true
	case "jvm.memory.heap.max":
		return true
	case "jvm.memory.heap.pool.committed":
		return true
	case "jvm.memory.heap.pool.max":
		return true
	case "jvm.memory.heap.pool.used":
		return true
	case "jvm.memory.heap.used":
		return true
	case "jvm.memory.non_heap.committed":
		return true
	case "jvm.memory.non_heap.max":
		return true
	case "jvm.memory.non_heap.used":
		return true
	case "jvm.thread.count":
		return true
	case "jvm.memory.non_heap.pool.used":
		return true
	case "jvm.memory.non_heap.pool.committed":
		return true
	case "jvm.memory.non_heap.pool.max":
		return true
	case "jvm.fd.used":
		return true
	case "jvm.fd.max":
		return true
	case "nodejs.eventloop.delay.avg.ms":
		return true
	case "nodejs.handles.active":
		return true
	case "nodejs.memory.arrayBuffers.bytes":
		return true
	case "nodejs.memory.external.bytes":
		return true
	case "nodejs.memory.heap.allocated.bytes":
		return true
	case "nodejs.memory.heap.used.bytes":
		return true
	case "nodejs.requests.active":
		return true
	case "ruby.gc.count":
		return true
	case "ruby.gc.time":
		return true
	case "ruby.heap.allocations.total":
		return true
	case "ruby.heap.slots.free":
		return true
	case "ruby.heap.slots.live":
		return true
	case "ruby.threads":
		return true
	case "system.cpu.total.norm.pct":
		return true
	case "system.memory.actual.free":
		return true
	case "system.memory.total":
		return true
	case "system.process.cgroup.cpu.cfs.period.us":
		return true
	case "system.process.cgroup.cpu.cfs.quota.us":
		return true
	case "system.process.cgroup.cpu.stats.periods":
		return true
	case "system.process.cgroup.cpu.stats.throttled.ns":
		return true
	case "system.process.cgroup.cpu.stats.throttled.periods":
		return true
	case "system.process.cgroup.cpuacct.total.ns":
		return true
	case "system.process.cgroup.memory.mem.limit.bytes":
		return true
	case "system.process.cgroup.memory.mem.usage.bytes":
		return true
	case "system.process.cgroup.memory.stats.inactive_file.bytes":
		return true
	case "system.process.cpu.system.norm.pct":
		return true
	case "system.process.cpu.total.norm.pct":
		return true
	case "system.process.cpu.user.norm.pct":
		return true
	case "system.process.memory.rss.bytes":
		return true
	case "system.process.memory.size":
		return true
	}
	return false
}
