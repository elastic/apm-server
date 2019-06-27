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

package apm

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"go.elastic.co/apm/internal/apmconfig"
	"go.elastic.co/apm/internal/wildcard"
)

const (
	envMetricsInterval       = "ELASTIC_APM_METRICS_INTERVAL"
	envMaxSpans              = "ELASTIC_APM_TRANSACTION_MAX_SPANS"
	envTransactionSampleRate = "ELASTIC_APM_TRANSACTION_SAMPLE_RATE"
	envSanitizeFieldNames    = "ELASTIC_APM_SANITIZE_FIELD_NAMES"
	envCaptureHeaders        = "ELASTIC_APM_CAPTURE_HEADERS"
	envCaptureBody           = "ELASTIC_APM_CAPTURE_BODY"
	envServiceName           = "ELASTIC_APM_SERVICE_NAME"
	envServiceVersion        = "ELASTIC_APM_SERVICE_VERSION"
	envEnvironment           = "ELASTIC_APM_ENVIRONMENT"
	envSpanFramesMinDuration = "ELASTIC_APM_SPAN_FRAMES_MIN_DURATION"
	envActive                = "ELASTIC_APM_ACTIVE"
	envAPIRequestSize        = "ELASTIC_APM_API_REQUEST_SIZE"
	envAPIRequestTime        = "ELASTIC_APM_API_REQUEST_TIME"
	envAPIBufferSize         = "ELASTIC_APM_API_BUFFER_SIZE"
	envMetricsBufferSize     = "ELASTIC_APM_METRICS_BUFFER_SIZE"
	envDisableMetrics        = "ELASTIC_APM_DISABLE_METRICS"

	defaultAPIRequestSize        = 750 * apmconfig.KByte
	defaultAPIRequestTime        = 10 * time.Second
	defaultAPIBufferSize         = 1 * apmconfig.MByte
	defaultMetricsBufferSize     = 100 * apmconfig.KByte
	defaultMetricsInterval       = 30 * time.Second
	defaultMaxSpans              = 500
	defaultCaptureHeaders        = true
	defaultCaptureBody           = CaptureBodyOff
	defaultSpanFramesMinDuration = 5 * time.Millisecond

	minAPIBufferSize     = 10 * apmconfig.KByte
	maxAPIBufferSize     = 100 * apmconfig.MByte
	minAPIRequestSize    = 1 * apmconfig.KByte
	maxAPIRequestSize    = 5 * apmconfig.MByte
	minMetricsBufferSize = 10 * apmconfig.KByte
	maxMetricsBufferSize = 100 * apmconfig.MByte
)

var (
	defaultSanitizedFieldNames = apmconfig.ParseWildcardPatterns(strings.Join([]string{
		"password",
		"passwd",
		"pwd",
		"secret",
		"*key",
		"*token*",
		"*session*",
		"*credit*",
		"*card*",
		"authorization",
		"set-cookie",
	}, ","))
)

func initialRequestDuration() (time.Duration, error) {
	return apmconfig.ParseDurationEnv(envAPIRequestTime, defaultAPIRequestTime)
}

func initialMetricsInterval() (time.Duration, error) {
	return apmconfig.ParseDurationEnv(envMetricsInterval, defaultMetricsInterval)
}

func initialMetricsBufferSize() (int, error) {
	size, err := apmconfig.ParseSizeEnv(envMetricsBufferSize, defaultMetricsBufferSize)
	if err != nil {
		return 0, err
	}
	if size < minMetricsBufferSize || size > maxMetricsBufferSize {
		return 0, errors.Errorf(
			"%s must be at least %s and less than %s, got %s",
			envMetricsBufferSize, minMetricsBufferSize, maxMetricsBufferSize, size,
		)
	}
	return int(size), nil
}

func initialAPIBufferSize() (int, error) {
	size, err := apmconfig.ParseSizeEnv(envAPIBufferSize, defaultAPIBufferSize)
	if err != nil {
		return 0, err
	}
	if size < minAPIBufferSize || size > maxAPIBufferSize {
		return 0, errors.Errorf(
			"%s must be at least %s and less than %s, got %s",
			envAPIBufferSize, minAPIBufferSize, maxAPIBufferSize, size,
		)
	}
	return int(size), nil
}

func initialAPIRequestSize() (int, error) {
	size, err := apmconfig.ParseSizeEnv(envAPIRequestSize, defaultAPIRequestSize)
	if err != nil {
		return 0, err
	}
	if size < minAPIRequestSize || size > maxAPIRequestSize {
		return 0, errors.Errorf(
			"%s must be at least %s and less than %s, got %s",
			envAPIRequestSize, minAPIRequestSize, maxAPIRequestSize, size,
		)
	}
	return int(size), nil
}

func initialMaxSpans() (int, error) {
	value := os.Getenv(envMaxSpans)
	if value == "" {
		return defaultMaxSpans, nil
	}
	max, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse %s", envMaxSpans)
	}
	return max, nil
}

// initialSampler returns a nil Sampler if all transactions should be sampled.
func initialSampler() (Sampler, error) {
	value := os.Getenv(envTransactionSampleRate)
	if value == "" || value == "1.0" {
		return nil, nil
	}
	ratio, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", envTransactionSampleRate)
	}
	if ratio < 0.0 || ratio > 1.0 {
		return nil, errors.Errorf(
			"invalid %s value %s: out of range [0,1.0]",
			envTransactionSampleRate, value,
		)
	}
	return NewRatioSampler(ratio), nil
}

func initialSanitizedFieldNames() wildcard.Matchers {
	return apmconfig.ParseWildcardPatternsEnv(envSanitizeFieldNames, defaultSanitizedFieldNames)
}

func initialCaptureHeaders() (bool, error) {
	return apmconfig.ParseBoolEnv(envCaptureHeaders, defaultCaptureHeaders)
}

func initialCaptureBody() (CaptureBodyMode, error) {
	value := os.Getenv(envCaptureBody)
	if value == "" {
		return defaultCaptureBody, nil
	}
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "all":
		return CaptureBodyAll, nil
	case "errors":
		return CaptureBodyErrors, nil
	case "transactions":
		return CaptureBodyTransactions, nil
	case "off":
		return CaptureBodyOff, nil
	}
	return -1, errors.Errorf("invalid %s value %q", envCaptureBody, value)
}

func initialService() (name, version, environment string) {
	name = os.Getenv(envServiceName)
	version = os.Getenv(envServiceVersion)
	environment = os.Getenv(envEnvironment)
	if name == "" {
		name = filepath.Base(os.Args[0])
		if runtime.GOOS == "windows" {
			name = strings.TrimSuffix(name, filepath.Ext(name))
		}
	}
	name = sanitizeServiceName(name)
	return name, version, environment
}

func initialSpanFramesMinDuration() (time.Duration, error) {
	return apmconfig.ParseDurationEnv(envSpanFramesMinDuration, defaultSpanFramesMinDuration)
}

func initialActive() (bool, error) {
	return apmconfig.ParseBoolEnv(envActive, true)
}

func initialDisabledMetrics() wildcard.Matchers {
	return apmconfig.ParseWildcardPatternsEnv(envDisableMetrics, nil)
}
