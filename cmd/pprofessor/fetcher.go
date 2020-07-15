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
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/google/pprof/driver"
	"github.com/google/pprof/profile"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/elastic/go-elasticsearch/v7/esutil"
)

const (
	indexPattern = "apm-*"
)

type fetcher struct {
	flags       driver.FlagSet
	startTime   *string
	serviceName *string
}

const fetcherUsage = `    -start                Start time for profile aggregation (defaults to now()-duration)
    -service              Profiled service name`

func newFetcher(flags driver.FlagSet) driver.Fetcher {
	flags.AddExtraUsage(fetcherUsage)
	return &fetcher{
		flags:       flags,
		startTime:   flags.String("start", "", "Start time for profile aggregation"),
		serviceName: flags.String("service", "", "Profiled service name"),
	}
}

// Fetch fetches a profile from Elasticsearch, by aggregating samples for a specified service name and time range.
func (f *fetcher) Fetch(src string, duration, timeout time.Duration) (*profile.Profile, string, error) {
	if *f.serviceName == "" {
		return nil, "", errors.New("-service is required")
	}
	if *f.startTime == "" && duration <= 0 {
		return nil, "", errors.New("one of -start or -duration is required")
	}

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// -start specified:               startTime=start, end=now
	// -duration specified:            startTime=now-duration, endTime=now
	// -start and -duration specified: startTime=start, endTime=start+duration
	startTime := *f.startTime
	endTime := "now"
	var durationString string
	if duration > 0 {
		durationString = fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	if startTime != "" {
		if durationString != "" {
			endTime = fmt.Sprintf("%s+%s", startTime, durationString)
		}
	} else {
		startTime = "now-" + durationString
	}

	// Honour the -tls_ca flag defined by pprof.
	var caCert []byte
	if flag := flag.Lookup("tls_ca"); flag != nil && flag.Value != nil {
		if filename := flag.Value.String(); filename != "" {
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				return nil, "", err
			}
			caCert = data
		}
	}

	cfg := elasticsearch.Config{
		Addresses: []string{src},
		CACert:    caCert,
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, "", err
	}
	p, err := f.fetchProfile(ctx, es, *f.serviceName, startTime, endTime)
	if err != nil {
		return nil, "", err
	}
	return p, src, nil
}

func (f *fetcher) fetchProfile(
	ctx context.Context,
	es *elasticsearch.Client,
	serviceName string,
	startTime, endTime string,
) (*profile.Profile, error) {

	stacksComposite := map[string]interface{}{
		"size": 1000,
		"sources": []map[string]interface{}{
			{"stack": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "profile.top.id",
				},
			}},
		},
	}

	profilesComposite := map[string]interface{}{
		"size": 1000,
		"sources": []map[string]interface{}{
			{"profile": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "profile.id",
				},
			}},
		},
	}

	// TODO(axw) query total duration of all profiles in the given time range.
	// To do that, we'll need to assign a unique ID to each profile, and then
	// perform a terms agg on the profile IDs, and sum up the duration of each.
	searchBody := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"processor.event": "profile"}},
					{"term": map[string]interface{}{"service.name": serviceName}},
					{"range": map[string]interface{}{
						"@timestamp": map[string]interface{}{
							"gte": startTime,
							"lte": endTime,
						},
					}},
				},
			},
		},
		"aggs": map[string]interface{}{
			"profiles": map[string]interface{}{
				"composite": profilesComposite,
				"aggs": map[string]interface{}{
					"duration_ns": map[string]interface{}{
						"max": map[string]interface{}{
							"field": "profile.duration",
						},
					},
				},
			},
			"stacks": map[string]interface{}{
				"composite": stacksComposite,
				"aggs": map[string]interface{}{
					"cpu_ns": map[string]interface{}{
						"sum": map[string]interface{}{
							"field": "profile.cpu.ns",
						},
					},
					"alloc_objects": map[string]interface{}{
						"sum": map[string]interface{}{
							"field": "profile.alloc_objects.count",
						},
					},
					"alloc_space": map[string]interface{}{
						"sum": map[string]interface{}{
							"field": "profile.alloc_space.bytes",
						},
					},
					"inuse_objects": map[string]interface{}{
						"sum": map[string]interface{}{
							"field": "profile.inuse_objects.count",
						},
					},
					"inuse_space": map[string]interface{}{
						"sum": map[string]interface{}{
							"field": "profile.inuse_space.bytes",
						},
					},
					"samples_count": map[string]interface{}{
						"sum": map[string]interface{}{
							"field": "profile.samples.count",
						},
					},
					"stack": map[string]interface{}{
						"top_hits": map[string]interface{}{
							"size": 1,
							"_source": map[string]interface{}{
								"includes": []interface{}{"profile.stack"},
							},
						},
					},
				},
			},
		},
	}

	profileNodes := make(map[string]*profileNode)
	rootProfileNodes := make(map[string]*profileNode)
	var totalDocs, totalProfiles int
	var totalDurationNanos int64
	for {
		req := esapi.SearchRequest{
			Index: []string{indexPattern},
			Body:  esutil.NewJSONReader(searchBody),
		}
		resp, err := req.Do(ctx, es)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.IsError() {
			return nil, errors.New(resp.String())
		}

		var result struct {
			Aggregations aggregationsResult
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		if len(result.Aggregations.Stacks.Buckets) == 0 &&
			len(result.Aggregations.Profiles.Buckets) == 0 {
			break
		}

		for _, profile := range result.Aggregations.Profiles.Buckets {
			totalProfiles++
			totalDocs += profile.DocCount
			totalDurationNanos += int64(profile.DurationNanos.Value)
		}

		for _, stack := range result.Aggregations.Stacks.Buckets {
			var prevNode *profileNode
			for i, frame := range stack.Stack.Hits.Hits[0].Source.Profile.Stack {
				node, ok := profileNodes[frame.ID]
				if !ok {
					node = &profileNode{
						id:       frame.ID,
						function: frame.Function,
						filename: frame.Filename,
						line:     int64(frame.Line),
						children: make(map[string]*profileNode),
					}
					profileNodes[frame.ID] = node
				}
				if i == 0 {
					node.samplesCount += int64(stack.SamplesCount.Value)
					node.cpuNanos += int64(stack.CPUNanos.Value)
					node.allocObjects += int64(stack.AllocObjects.Value)
					node.allocSpaceBytes += int64(stack.AllocSpace.Value)
					node.inuseObjects += int64(stack.InuseObjects.Value)
					node.inuseSpaceBytes += int64(stack.InuseSpace.Value)
				} else {
					node.children[prevNode.id] = prevNode
				}
				prevNode = node
			}
			if prevNode != nil {
				rootProfileNodes[prevNode.id] = prevNode
			}
		}

		// Paginate.
		profilesComposite["after"] = result.Aggregations.Profiles.AfterKey
		stacksComposite["after"] = result.Aggregations.Stacks.AfterKey
	}

	p := &profile.Profile{
		DurationNanos: totalDurationNanos,
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
			{Type: "inuse_space", Unit: "bytes"},
		},
		Mapping: []*profile.Mapping{
			// TODO(axw) have agents/server record Build ID, and
			// include in the mapping. Currently we do not discriminate
			// multiple builds. We should at least provide an indicator
			// that a profile spans multiple builds, when it does.
			{ID: 1, File: serviceName},
		},
		Comments: []string{
			fmt.Sprintf(
				"Aggregated from %d doc%s, %d profile%s",
				totalDocs, plural(totalDocs),
				totalProfiles, plural(totalProfiles),
			),
		},
	}

	locations := make(map[string]*profile.Location)
	functions := make(map[string]*profile.Function)
	for _, node := range profileNodes {
		if locations[node.id] != nil {
			continue
		}
		fn, ok := functions[node.function]
		if !ok {
			fn = &profile.Function{
				ID:   uint64(len(functions) + 1),
				Name: node.function,
			}
			p.Function = append(p.Function, fn)
			functions[node.function] = fn
		}
		loc := &profile.Location{
			ID:   uint64(len(locations) + 1),
			Line: []profile.Line{{Function: fn}},
		}
		p.Location = append(p.Location, loc)
		locations[node.id] = loc
	}

	var addSample func(node *profileNode, location []*profile.Location)
	addSample = func(node *profileNode, location []*profile.Location) {
		location = append([]*profile.Location{locations[node.id]}, location...)
		if node.samplesCount > 0 {
			p.Sample = append(p.Sample, &profile.Sample{
				Location: location,
				Value: []int64{
					node.samplesCount,
					node.cpuNanos,
					node.allocObjects,
					node.allocSpaceBytes,
					node.inuseObjects,
					node.inuseSpaceBytes,
				},
			})
		}
		for _, child := range node.children {
			addSample(child, location)
		}
	}
	for _, node := range rootProfileNodes {
		addSample(node, nil)
	}

	return p, p.CheckValid()
}

type aggregationsResult struct {
	Profiles struct {
		AfterKey interface{} `json:"after_key"`
		Buckets  []struct {
			Key struct {
				Profile string
			}
			DocCount      int             `json:"doc_count"`
			DurationNanos numericAggValue `json:"duration_ns"`
		}
	}
	Stacks struct {
		AfterKey interface{} `json:"after_key"`
		Buckets  []struct {
			Key struct {
				Stack string
			}
			DocCount     int             `json:"doc_count"`
			CPUNanos     numericAggValue `json:"cpu_ns"`
			AllocObjects numericAggValue `json:"alloc_objects"`
			AllocSpace   numericAggValue `json:"alloc_space"`
			InuseObjects numericAggValue `json:"inuse_objects"`
			InuseSpace   numericAggValue `json:"inuse_space"`
			SamplesCount numericAggValue `json:"samples_count"`
			Stack        struct {
				Hits struct {
					Hits []struct {
						Source struct {
							Profile struct {
								Stack []stackFrame
							}
						} `json:"_source"`
					}
				}
			}
		}
	}
}

type numericAggValue struct {
	Value float64
}

type stackFrame struct {
	ID       string
	Function string
	Filename string
	Line     float64
}

type profileNode struct {
	id       string
	function string
	filename string
	line     int64

	samplesCount    int64
	cpuNanos        int64
	allocObjects    int64
	allocSpaceBytes int64
	inuseObjects    int64
	inuseSpaceBytes int64
	children        map[string]*profileNode
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
