// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/apm-server/elasticsearch"
)

// getGlobalCheckpoints returns the current global checkpoint for each index
// underlying dataStream. Each index is required to have a single (primary) shard.
func getGlobalCheckpoints(
	ctx context.Context,
	client elasticsearch.Client,
	dataStream string,
) (map[string]int64, error) {
	indexGlobalCheckpoints := make(map[string]int64)
	resp, err := esapi.IndicesStatsRequest{
		Index: []string{dataStream},
		Level: "shards",
		// By default all metrics are returned; query just the "get" metric,
		// which is very cheap.
		Metric: []string{"get"},
	}.Do(ctx, client)
	if err != nil {
		return nil, errors.New("index stats request failed")
	}
	defer resp.Body.Close()
	if resp.IsError() {
		switch resp.StatusCode {
		case http.StatusNotFound:
			// Data stream does not yet exist.
			return indexGlobalCheckpoints, nil
		}
		message, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("index stats request failed: %s", message)
	}

	var stats dataStreamStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	for index, indexStats := range stats.Indices {
		if n := len(indexStats.Shards); n > 1 {
			return nil, fmt.Errorf("expected 1 shard, got %d for index %q", n, index)
		}
		for _, shardStats := range indexStats.Shards {
			for _, shardStats := range shardStats {
				if shardStats.Routing.Primary {
					indexGlobalCheckpoints[index] = shardStats.SeqNo.GlobalCheckpoint
					break
				}
			}
		}
	}
	return indexGlobalCheckpoints, nil
}

type dataStreamStats struct {
	Indices map[string]indexStats `json:"indices"`
}

type indexStats struct {
	Shards map[string][]shardStats `json:"shards"`
}

type shardStats struct {
	Routing struct {
		Primary bool `json:"primary"`
	} `json:"routing"`
	SeqNo struct {
		GlobalCheckpoint int64 `json:"global_checkpoint"`
	} `json:"seq_no"`
}
