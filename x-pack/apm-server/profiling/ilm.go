// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

/*
Here we manage the ILM for indices holding non-timeseries data,
such as stacktraces, stackframes and executables.

Details on why we need this implementation can be found at
https://github.com/elastic/elasticsearch/issues/85273.

We multiplex write each document into 2 separate aliases: "current" and "next".
Reads will only go to the "current" alias, which for simplicity has the suffix omitted.
At any given time, "<alias-index>" and "<alias-index>-next" are backed by 2 indices.

client ++++ --> <alias-index> 		--> <index-AAAA-BB-CC.hh.mm.ss>

	       	|
			+ --> <alias-index>-next 	--> <index-DDDD-EE-FF.hh.mm.ss>

We regularly check from the collector if it's time to apply our custom ILM policy and rollover.
The rollover happens by creating a new index and shifting both aliases by 1 towards the newest.
All the alias actions are bundled in a single atomic operation in Elasticsearch, documented at
https://www.elastic.co/guide/en/elasticsearch/reference/master/indices-aliases.html

	new-index-YYYY-MM <- - - - - - - -
	                                 |
	                                 |_ <alias-index>-next
	<index-DDDD-EE-FF.hh.mm.ss>  <- -
	                                 |
	<index-AAAA-BB-CC.hh.mm.ss>      |_ <alias-index>

Finally, the oldest index is deleted to free up space.
*/

package profiling

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/common"
	"github.com/elastic/apm-server/x-pack/apm-server/profiling/libpf"
)

var (
	ilmRegistry             = monitoring.Default.NewRegistry("apm-server.profiling.ilm")
	rolloverCounter         = monitoring.NewInt(ilmRegistry, "custom_ilm.execution.count")
	rolloverSkippedCounter  = monitoring.NewInt(ilmRegistry, "custom_ilm.skipped_for_time_constraints.count")
	rolloverFailuresCounter = monitoring.NewInt(ilmRegistry, "custom_ilm.failed.count")
	rolloverUndeletedIndex  = monitoring.NewInt(ilmRegistry, "custom_ilm.undeleted_index.count")
	keyValueIndices         = []string{common.StackTraceIndex, common.StackFrameIndex, common.ExecutablesIndex}
)

// ScheduleILMExecution creates goroutines to manage ILM with custom policy for key/value data:
// ILM would normally be managed inside Elasticsearch, but the peculiar nature of our data
// is still not part of an ES ILM logic yet (see ilm.go for more details).
//
// Each index has a separate goroutine managing it, the shutdown via context
// and ILM policy is shared across all goroutines.
func ScheduleILMExecution(ctx context.Context, logger *logp.Logger, cfg config.ProfilingConfig) {
	const (
		maxRetries = 4
		jitter     = 0.2
	)
	// create an ES client that can fulfill the ILM operations
	client, err := es.NewClient(es.Config{
		Addresses: cfg.ESConfig.Hosts,
		APIKey:    base64.StdEncoding.EncodeToString([]byte(cfg.ESConfig.APIKey)),
		// disable retries as we have dedicated retry mechanism
		DisableRetry: true,
	})
	if err != nil {
		logger.Errorf("Can't create client to perform custom ILM strategy: %v", err)
		return
	}

	policy := ilmPolicy{
		cfg.ILMConfig.SizeInBytes,
		cfg.ILMConfig.Age,
	}
	for _, idx := range keyValueIndices {
		go func(index string) {
			// Jitter and the atomicity of rollover/swap give enough guarantee that
			// we avoid collisions between multiple instances of the collector.
			// A more robust concurrency control is implemented in the rollover function.
			randomizedTicker := time.NewTicker(libpf.AddJitter(cfg.ILMConfig.Interval, jitter))
			runner := &optimisticAliasSwap{
				client:       client,
				currentAlias: index,
				logger:       logger,
			}
			for {
				// runILM is a blocking call, waiting for the ticker's tick or context expiration.
				if err := runILM(ctx, randomizedTicker.C, maxRetries, runner, policy); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					logger.Warnf("Custom ILM strategy: %v", err)
				}
				randomizedTicker.Reset(libpf.AddJitter(cfg.ILMConfig.Interval, jitter))
			}
		}(idx)
	}
}

// ILMRunner describes the behavior of a custom ILM strategy executor
type ILMRunner interface {
	configure() error
	preFlight(policy ilmPolicy) error
	execute(policy ilmPolicy) error
}

type optimisticAliasSwap struct {
	client *es.Client
	currentAlias,
	nextIndex,
	nextAlias,
	currentIndex string

	logger *logp.Logger
}

func (o *optimisticAliasSwap) configure() error {
	return bootstrapILMLockDocument(o.client, o.currentAlias)
}

func (o *optimisticAliasSwap) preFlight(policy ilmPolicy) error {
	var err error
	o.currentIndex, err = indexFromAlias(o.client, o.currentAlias)
	if err != nil {
		return err
	}

	if !indexShouldBeRolledOver(o.logger, o.client, o.currentIndex, policy) {
		return errPreFlightFailed{o.currentIndex}
	}

	o.nextAlias = nextIndex(o.currentAlias)
	if o.nextIndex, err = indexFromAlias(o.client, o.nextAlias); err != nil {
		return err
	}

	o.logger.With(logp.String("index", o.currentAlias)).
		Debugf("Index status matching the ILM policy %s, possibly start roll-over", policy)

	return nil
}

func (o *optimisticAliasSwap) execute(policy ilmPolicy) error {
	ok, err := ilmAcquireLock(o.client, o.currentAlias, policy.age)
	if err != nil {
		return err
	}
	if !ok {
		// Early-exit, some other replica has acquired the lock
		return nil
	}
	// Releasing the lock does not need to check on errors,
	// because the lock acquire process is self-healing, based on the timerange
	defer func() {
		if err = ilmIndexLockDocument(o.client, o.currentAlias, "completed",
			nil, nil); err != nil {
			o.logger.Warnf("Failed to index document in lock index: %v", err)
		}
	}()
	rolloverCounter.Inc()

	// Create a new index: setting the right name will allow inheriting
	// the configuration from the template
	newIdx := fmt.Sprintf("%s-%s", o.currentAlias,
		time.Now().UTC().Format("2006.02.01-15.04.05"))
	resp, err := o.client.Indices.Create(newIdx)
	if checkESAPIError(http.StatusOK,
		resp, err) != nil {
		rolloverFailuresCounter.Inc()
		return fmt.Errorf("creating new index %s failed: %v", newIdx, err)
	}
	defer resp.Body.Close()

	// Update the aliases to the new window in an atomic operation
	buf := bytes.NewBufferString(fmt.Sprintf(`{
	"actions": [
		{
			"remove": {
				"index": "%s",
				"alias": "%s"
			}
		},
		{
			"add": {
				"index": "%s",
				"alias": "%s",
				"is_write_index": true
			}
		},
		{
			"remove": {
				"index": "%s",
				"alias": "%s"
			}
		},
		{
			"add": {
				"index": "%s",
				"alias": "%s",
				"is_write_index": true
			}
		}
	]}`, o.nextIndex, o.nextAlias,
		newIdx, o.nextAlias,
		o.currentIndex, o.currentAlias,
		o.nextIndex, o.currentAlias))
	resp, err = o.client.Indices.UpdateAliases(buf)
	if checkESAPIError(http.StatusOK,
		resp, err) != nil {
		return fmt.Errorf("flipping aliases failed for target %s: %v", o.currentAlias, err)
	}
	defer resp.Body.Close()

	// Finally, delete the index previously backing the "current" alias.
	// We don't return an error here, simply log it, because we don't want to re-execute
	// the function in case the old index is not deleted.
	//
	// There is no guarantee that we actually free space with this delete API call,
	// therefore we should keep an eye on deleting all previous indices if this
	// implementation is deployed in production by users.
	if resp, err := o.client.Indices.Delete([]string{o.currentIndex}); err != nil ||
		resp.StatusCode != http.StatusOK {
		rolloverUndeletedIndex.Inc()
		o.logger.With(logp.String("index", o.currentIndex)).
			Errorf("Deleting old index failed: %v", err)
	}
	return nil
}

type ilmPolicy struct {
	sizeInBytes uint64
	age         time.Duration
}

func (c ilmPolicy) String() string {
	return fmt.Sprintf("{ age: %s, size: %s }", c.age, humanize.Bytes(c.sizeInBytes))
}

type errPreFlightFailed struct {
	index string
}

func (e errPreFlightFailed) Error() string {
	return "index " + e.index + " is not suitable for rollover"
}

// Time-based execution of custom ILM policy
func runILM(ctx context.Context, tick <-chan time.Time, maxRetry uint32,
	ilm ILMRunner, policy ilmPolicy) error {
	select {
	case <-tick:
		// Configuring the ILM runner will not be retried on error: the method should be idempotent
		if err := ilm.configure(); err != nil {
			return fmt.Errorf("configure failed: %v", err)
		}
		// If preflight checks fail, we don't want to retry or continue this execution
		if err := ilm.preFlight(policy); err != nil {
			if errors.As(err, &errPreFlightFailed{}) {
				return nil
			}
			return fmt.Errorf("pre-flight failed: %v", err)
		}
		if err := ilm.execute(policy); err != nil {
			var retryErr error
			for i := uint32(0); i < maxRetry; i++ {
				if retryErr = ilm.execute(policy); retryErr == nil {
					return fmt.Errorf("execute failed: %v", err)
				}
			}
			return retryErr
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// Writing or updating a document in the locking index with refresh=true will have
// the index refreshed immediately, thus reducing the time a shard may reply with an older version.
// We expect that a second query to acquire the lock (possibly running on another shard)
// will not be able to run successfully because the sequence was updated by another client.
//
// The parameters passed will allow to manage the document lifecycle: their Go zero-value
// should be used in the caller to adopt the client's default behavior.
// seqNo, primaryTerm control whether the update tries to use optimistic concurrency control.
func ilmIndexLockDocument(client *es.Client, targetAlias, phase string,
	seqNo, primaryTerm *int) error {
	clientOpts := []func(request *esapi.IndexRequest){
		client.Index.WithDocumentID(targetAlias),
		client.Index.WithRefresh("true"),	// TODO: review stateless elasticsearch impact
	}
	if seqNo != nil && primaryTerm != nil {
		clientOpts = append(clientOpts,
			client.Index.WithIfSeqNo(*seqNo),
			client.Index.WithIfPrimaryTerm(*primaryTerm))
	}

	body := bytes.NewBufferString(fmt.Sprintf(`{
	"@timestamp": %d,
	"phase": "%s"
}`, time.Now().UTC().Unix(), phase))
	r, err := client.Index(common.ILMLockingIndex,
		body,
		clientOpts...)
	if checkESAPIError(http.StatusOK, r, err) != nil {
		return fmt.Errorf("unable to write lock document for target %s: %v", targetAlias, err)
	}
	r.Body.Close()
	return nil
}

func ilmAcquireLock(client *es.Client, targetAlias string, minElapsed time.Duration) (bool, error) {
	// When the timestamp of ILM rollover for the target index is not as old as the index,
	// we don't acquire a lock, because it means a rollover was applied already.
	// If pre-flight checks are ok, we acquire a lock and signal other replicas about it.
	resp, err := client.Get(common.ILMLockingIndex, targetAlias)
	if checkESAPIError(http.StatusOK, resp, err) != nil {
		return false, fmt.Errorf("unable to query Get API for alias: %v", err)
	}
	defer resp.Body.Close()

	var getResp GetAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return false, fmt.Errorf("decoding GetAPIResponse struct failed: %v", err)
	}
	if int64(getResp.Source.Timestamp) > time.Now().Add(-minElapsed).Unix() {
		rolloverSkippedCounter.Inc()
		return false, nil
	}
	// Verify that concurrent requests are non-conflicting,
	// using seq_no and primary_term from the previous read
	if err := ilmIndexLockDocument(client, targetAlias, "in_progress",
		&getResp.SeqNo, &getResp.PrimaryTerm); err != nil {
		return false, nil
	}
	return true, nil
}

type GetAPIResponse struct {
	Index       string `json:"_index"`
	ID          string `json:"_id"`
	Version     int    `json:"_version"`
	SeqNo       int    `json:"_seq_no"`
	PrimaryTerm int    `json:"_primary_term"`
	Found       bool   `json:"found"`
	Source      struct {
		Timestamp int    `json:"@timestamp"`
		Phase     string `json:"phase"`
	} `json:"_source"`
}

// Checks the status of the index against the cat API.
// We use the cat API to avoid parsing the Index API response raw JSON,
// the Go Elasticsearch client does not have a type for it.
func indexShouldBeRolledOver(logger *logp.Logger, client *es.Client, currentIdx string,
	policy ilmPolicy) bool {
	indicesResp, err := client.Cat.Indices(
		client.Cat.Indices.WithIndex(currentIdx),
		client.Cat.Indices.WithH("pri", "pri.store.size", "creation.date.string"),
	)
	defer indicesResp.Body.Close()
	if checkESAPIError(http.StatusOK, indicesResp, err) != nil {
		logger.With(logp.String("index", currentIdx)).
			Errorf("Unable to query cat API: %v", err)
		return false
	}
	return checkPolicy(logger, indicesResp.Body, policy)
}

func indexFromAlias(client *es.Client, alias string) (string, error) {
	resp, err := client.Indices.GetAlias(client.Indices.GetAlias.WithIndex(alias))
	if checkESAPIError(http.StatusOK, resp, err) != nil {
		return "", fmt.Errorf("unable to discover index for alias %s: %v", alias, err)
	}
	parsed := make(map[string]interface{})
	if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("converting GetAlias JSON response: %v", err)
	}
	resp.Body.Close()
	if len(parsed) > 1 {
		return "", fmt.Errorf("expected only 1 index for alias %s", alias)
	}
	var theIndex string
	for theIndex = range parsed {
		return theIndex, nil
	}
	return "", fmt.Errorf("found no index for alias %s", alias)
}

func checkPolicy(logger *logp.Logger, catResponse io.ReadCloser, policy ilmPolicy) bool {
	defer catResponse.Close()
	s := bufio.NewScanner(catResponse)
	for s.Scan() {
		text := s.Text()
		if text == "" {
			continue
		}
		line := strings.Fields(strings.TrimSpace(text))
		primaries, primariesSize, creationDate := line[0], line[1], line[2]
		numPrimaries, err := strconv.ParseUint(primaries, 10, 64)
		if err != nil {
			logger.Errorf("Parsing number of indices from _cat: %v", err)
			return false
		}
		primariesByteSize, err := humanize.ParseBytes(primariesSize)
		if err != nil {
			logger.Errorf("Parsing size in bytes from _cat: %v", err)
			return false
		}
		createDate, err := time.Parse(time.RFC3339Nano, creationDate)
		if err != nil {
			logger.Errorf("Parsing time from _cat: %v", err)
			return false
		}
		if primariesByteSize/numPrimaries > policy.sizeInBytes ||
			time.Since(createDate) > policy.age {
			logger.Debugf("Rolling over because size is %s and age is %s",
				humanize.Bytes(primariesByteSize), time.Since(createDate))
			return true
		}
	}
	return false
}

func checkESAPIError(statusCode int, resp *esapi.Response, err error) error {
	if err != nil {
		return fmt.Errorf("ES API client error: %v", err)
	}
	if resp.StatusCode != statusCode {
		return fmt.Errorf("ES API response yielded unexpected result: want %d, got %d: %s",
			statusCode, resp.StatusCode, resp.String())
	}
	return nil
}

func bootstrapILMLockDocument(client *es.Client, targetAlias string) error {
	body := bytes.NewBufferString(fmt.Sprintf(`{
	"@timestamp": %d,
	"phase": "%s"
}`, time.Now().UTC().Unix(), "pending"))
	// Create the document the first time, when the index is empty
	// Conflict: another replica has already created the first document
	// for the same index
	r, err := client.Index(common.ILMLockingIndex,
		body, client.Index.WithDocumentID(targetAlias),
		client.Index.WithRefresh("true"),	// TODO: review stateless elasticsearch impact
		client.Index.WithOpType("create"),
	)
	defer r.Body.Close()

	if checkESAPIError(http.StatusConflict, r, err) == nil {
		return nil
	}
	return checkESAPIError(http.StatusCreated, r, err)
}

func nextIndex(index string) string {
	return index + "-next"
}
