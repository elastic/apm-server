// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

// getGlobalCheckpoints returns the current global checkpoint for each index
// underlying dataStream. Each index is required to have a single (primary) shard.
func getGlobalCheckpoints(
	ctx context.Context,
	client *elastictransport.Client,
	dataStream string,
) (map[string]int64, error) {
	indexGlobalCheckpoints := make(map[string]int64)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/"+dataStream+"/_stats/get?level=shards", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create index stats request: %w", err)
	}
	resp, err := client.Perform(req)
	if err != nil {
		return nil, fmt.Errorf("index stats request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		switch resp.StatusCode {
		case http.StatusNotFound:
			// Data stream does not yet exist.
			return indexGlobalCheckpoints, nil
		}
		message, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("index stats request failed with status code %d: %s", resp.StatusCode, message)
	}

	var stats dataStreamStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to parse index stats response: %w", err)
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
