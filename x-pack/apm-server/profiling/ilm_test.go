// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/logp"
	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type stubErrCountILM struct{ counter *uint32 }
type stubOkCountILM struct{ counter *uint32 }

func (ilm *stubErrCountILM) execute(ilmPolicy) error {
	atomic.AddUint32(ilm.counter, 1)
	return fmt.Errorf("anything")
}
func (ilm *stubErrCountILM) configure() error          { return nil }
func (ilm *stubErrCountILM) preFlight(ilmPolicy) error { return nil }

func (ilm *stubOkCountILM) execute(ilmPolicy) error {
	atomic.AddUint32(ilm.counter, 1)
	return nil
}
func (ilm *stubOkCountILM) configure() error          { return nil }
func (ilm *stubOkCountILM) preFlight(ilmPolicy) error { return nil }

func TestApplyILMOnTicker_ResumeAtNextTickOnExecutionError(t *testing.T) {
	applyILM := time.NewTicker(1 * time.Millisecond)
	defer applyILM.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	counter := new(uint32)
	failure := &stubErrCountILM{counter: counter}

	for {
		err := runILM(ctx, applyILM.C, 1, failure, ilmPolicy{})
		if errors.Is(err, context.DeadlineExceeded) {
			break
		}
		require.NotNil(t, err)
		assert.Equal(t, "anything", err.Error())
	}
	assert.True(t, *counter > 1)
}

func TestApplyILMOnTicker_RetryAttemptsWhenFailing(t *testing.T) {
	// prepare input values: we call twice runILM,
	// so we send 2 values in the ticker chan to trigger execution
	const runAttempts = 2
	ticker := make(chan time.Time, runAttempts)
	go func() {
		for i := 0; i < runAttempts; i++ {
			ticker <- time.Now().UTC()
		}
	}()

	counter := new(uint32)
	failure := &stubErrCountILM{counter: counter}
	success := &stubOkCountILM{counter: counter}

	retries := uint32(10)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	expectedErr := runILM(ctx, ticker, retries, failure, ilmPolicy{})
	require.NotNil(t, expectedErr)
	assert.Equal(t, "anything", expectedErr.Error())
	// The first execution is _before_ the retries, so when the ILMRunner func fails
	// we re-execute up to maxRetry
	assert.Equal(t, retries+1, *counter)

	// When a function is successful, we only expect it to run once
	atomic.SwapUint32(counter, 0)
	err := runILM(ctx, ticker, 100, success, ilmPolicy{})
	require.Nil(t, err)
	assert.Equal(t, uint32(1), *counter)
}

func TestApplyILM_CustomPolicyIsAppliedOnlyIfPreconditionIsValid(t *testing.T) {
	policy := ilmPolicy{
		sizeInBytes: 1 << 8,
		age:         30 * 24 * time.Hour,
	}
	twoDaysAgo := time.Now().Add(-2 * 24 * time.Hour)
	applyILM := time.NewTicker(3 * time.Millisecond)
	defer applyILM.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	conditionNotMet := func(ilmPolicy ilmPolicy) ILMRunner {
		if time.Since(twoDaysAgo) >= ilmPolicy.age {
			return &stubErrCountILM{counter: new(uint32)}
		}
		return &stubOkCountILM{counter: new(uint32)}
	}

	err := runILM(ctx, applyILM.C, 100, conditionNotMet(policy), ilmPolicy{})
	assert.Nil(t, err)
}

func TestApplyILM_ParseCatIndicesOutput(t *testing.T) {
	type testCase struct {
		name     string
		text     *bytes.Buffer
		policy   ilmPolicy
		expected bool
	}
	const hundredYears = 100 * 365 * 24 * time.Hour
	tests := []testCase{
		{
			name: "2_indices_oly_one_is_exceeding_size_and_date",
			text: bytes.NewBufferString(`
1 3.5kb 2022-04-29T07:53:16.429Z
1  51gb 2022-01-01T00:00:00.123Z
`),
			policy: ilmPolicy{
				sizeInBytes: 50 << 30,
				age:         2 * time.Minute,
			},
			expected: true,
		}, {
			name: "1_index_exceeding_size_with_empty_line",
			text: bytes.NewBufferString(`
1 3.5kb 2022-04-29T07:53:16.429Z

`),
			policy: ilmPolicy{
				// 3 Kib
				sizeInBytes: 3 << 10,
				age:         10 * 24 * time.Hour,
			},
			expected: true,
		}, {
			name: "1_index_exceeding_only_date",
			text: bytes.NewBufferString(`
16 35gb 2022-04-29T07:53:16.429Z
16 35kb 2222-04-29T07:53:16.429Z
`),
			policy: ilmPolicy{
				sizeInBytes: 50 << 30,
				age:         24 * time.Hour,
			},
			expected: true,
		}, {
			name: "no_index_exceeding",
			text: bytes.NewBufferString(`
1 49.9gb 2022-04-29T07:53:16.429Z
1   35kb 2022-04-29T07:53:16.429Z
`),
			policy: ilmPolicy{
				sizeInBytes: 50 << 30,
				age:         hundredYears,
			},
			expected: false,
		}, {
			name: "no_index_exceeding_with_tabs",
			text: bytes.NewBufferString(`
1 	5.8mb 2022-05-03T09:49:10.800Z
1 267.2kb 2022-05-03T09:49:09.617Z
`),
			policy: ilmPolicy{
				sizeInBytes: 50 << 30,
				age:         hundredYears,
			},
			expected: false,
		}, {
			name: "index_exceeding_on_per_shard_size",
			text: bytes.NewBufferString(`
10 13mb 2022-05-03T09:49:10.800Z
`),
			policy: ilmPolicy{
				// 1 MiB is less than 1.3 MB (13mb / 10 shards),
				// so we should rollover
				sizeInBytes: 1 << 20,
				age:         hundredYears,
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, checkPolicy(logp.NewLogger("test"),
				io.NopCloser(tc.text), tc.policy))
		})
	}
}

func TestILMLockAcquire(t *testing.T) {
	indexName := "index_test"
	now := time.Now().UTC()
	minElapsed := time.Since(now.Add(-10 * time.Minute))
	inRange := now.Add(-5 * time.Second)
	outsideRange := now.Add(-100 * time.Minute)
	testCases := []struct {
		name       string
		mockedResp string
		expected   bool
	}{
		{
			name: "cant_acquire_inprogress_lock_in_timerange",
			mockedResp: fmt.Sprintf(docLockInProgressFmt,
				indexName, inRange.Unix(), "in_progress"),
			expected: false,
		}, {
			name: "cant_acquire_completed_lock_in_timerange",
			mockedResp: fmt.Sprintf(docLockInProgressFmt,
				indexName, inRange.Unix(), "completed"),
			expected: false,
		}, {
			name: "can_acquire_in_progress_lock_outside_timerange",
			mockedResp: fmt.Sprintf(docLockCompletedFmt,
				indexName, outsideRange.Unix(), "in_progress"),
			expected: true,
		}, {
			name: "can_acquire_completed_lock_outside_timerange",
			mockedResp: fmt.Sprintf(docLockCompletedFmt,
				indexName, outsideRange.Unix(), "completed"),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// response we get from ES when an index is being rolled over by
			// any instance of the collector
			cfg := es.Config{
				Addresses: []string{"http://any.url:1234"},
				APIKey:    "apiKey",
				Transport: http.DefaultTransport,
			}
			client, err := es.NewClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			client.Transport = mockESClient{
				stubResp: &http.Response{
					Body:       io.NopCloser(bytes.NewBufferString(tc.mockedResp)),
					StatusCode: http.StatusOK,
					Status:     "::anything::",
					Header:     map[string][]string{"X-Elastic-Product": {"Elasticsearch"}},
				},
				testFn: func(req *http.Request) bool {
					return assert.Contains(t, req.URL.String(), indexName)
				},
			}
			ok, err := ilmAcquireLock(client, indexName, minElapsed)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestParseLockIndexResponse(t *testing.T) {
	var resp GetAPIResponse
	now := time.Now().Unix()
	indexName := "index_name"
	data := fmt.Sprintf(docLockCompletedFmt, indexName, now, "completed")
	require.Nil(t, json.NewDecoder(bytes.NewBufferString(data)).Decode(&resp))
	assert.Equal(t, now, int64(resp.Source.Timestamp))
	assert.Equal(t, indexName, resp.ID)
}

var (
	docLockCompletedFmt = `{
  "_index": ".profiling-ilm-lock",
  "_id": "%s",
  "_version": 3,
  "_seq_no": 44,
  "_primary_term": 1,
  "found": true,
  "_source": {
    "@timestamp": %d,
    "phase": "%s"
  }
}`
	docLockInProgressFmt = `{
  "_index": ".profiling-ilm-lock",
  "_id": "%s",
  "_version": 5,
  "_seq_no": 46,
  "_primary_term": 1,
  "found": true,
  "_source": {
    "@timestamp": %d,
    "phase": "%s"
  }
}`
)

func TestCheckErrorFromElasticsearchAPI(t *testing.T) {
	const errText = "some error text"
	testCases := []struct {
		name               string
		err                error
		resp               *esapi.Response
		expectedStatusCode int
		wantErr            bool
	}{
		{
			"nil_error_with_expected_code",
			nil,
			&esapi.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(errText))},
			http.StatusOK,
			false,
		}, {
			"nil_error_with_unexpected_code",
			nil,
			&esapi.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString(errText))},
			http.StatusOK,
			true,
		}, {
			"non_nil_error_with_expected_code",
			errors.New(errText),
			&esapi.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(errText))},
			http.StatusOK,
			true,
		}, {
			"non_nil_error_with_unexpected_code",
			errors.New(errText),
			&esapi.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString(errText))},
			http.StatusOK,
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := checkESAPIError(tc.expectedStatusCode, tc.resp, tc.err)
			if tc.wantErr {
				if tc.resp.StatusCode != tc.expectedStatusCode && tc.err == nil {
					assert.Contains(t, err.Error(), strconv.Itoa(tc.resp.StatusCode))
				}
				assert.NotNil(t, err)
				assert.True(t, strings.HasSuffix(err.Error(), errText))
				return
			}
			assert.Nil(t, err)
		})
	}
}

type dummyPreflight struct{}

func (d dummyPreflight) configure() error {
	return nil
}

func (d dummyPreflight) preFlight(ilmPolicy) error {
	return errPreFlightFailed{"test"}
}

func (d dummyPreflight) execute(ilmPolicy) error {
	return nil
}

func Test_PreflightCheckReturnsTypedError(t *testing.T) {
	var underTest ILMRunner = dummyPreflight{}

	assert.IsType(t, underTest.preFlight(ilmPolicy{}), errPreFlightFailed{})
	assert.True(t, errors.As(underTest.preFlight(ilmPolicy{}), &errPreFlightFailed{}))
}

type mockESClient struct {
	stubResp *http.Response
	testFn   func(req *http.Request) bool
}

// Perform mocks the client call to ElasticSearch
func (m mockESClient) Perform(req *http.Request) (*http.Response, error) {
	if m.testFn(req) {
		return m.stubResp, nil
	}
	return nil, fmt.Errorf("wrong JSON in request body")
}
