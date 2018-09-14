package elasticapm

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/internal/apmconfig"
)

const (
	envFlushInterval         = "ELASTIC_APM_FLUSH_INTERVAL"
	envMetricsInterval       = "ELASTIC_APM_METRICS_INTERVAL"
	envMaxQueueSize          = "ELASTIC_APM_MAX_QUEUE_SIZE"
	envMaxSpans              = "ELASTIC_APM_TRANSACTION_MAX_SPANS"
	envTransactionSampleRate = "ELASTIC_APM_TRANSACTION_SAMPLE_RATE"
	envSanitizeFieldNames    = "ELASTIC_APM_SANITIZE_FIELD_NAMES"
	envCaptureBody           = "ELASTIC_APM_CAPTURE_BODY"
	envServiceName           = "ELASTIC_APM_SERVICE_NAME"
	envServiceVersion        = "ELASTIC_APM_SERVICE_VERSION"
	envEnvironment           = "ELASTIC_APM_ENVIRONMENT"
	envSpanFramesMinDuration = "ELASTIC_APM_SPAN_FRAMES_MIN_DURATION"
	envActive                = "ELASTIC_APM_ACTIVE"
	envDistributedTracing    = "ELASTIC_APM_DISTRIBUTED_TRACING"

	defaultFlushInterval           = 10 * time.Second
	defaultMetricsInterval         = 0 // disabled by default
	defaultMaxTransactionQueueSize = 500
	defaultMaxSpans                = 500
	defaultCaptureBody             = CaptureBodyOff
	defaultSpanFramesMinDuration   = 5 * time.Millisecond
)

var (
	defaultSanitizedFieldNames = regexp.MustCompile(fmt.Sprintf("(?i:%s)", strings.Join([]string{
		"password",
		"passwd",
		"pwd",
		"secret",
		".*key",
		".*token",
		".*session.*",
		".*credit.*",
		".*card.*",
	}, "|")))
)

func initialFlushInterval() (time.Duration, error) {
	return apmconfig.ParseDurationEnv(envFlushInterval, "s", defaultFlushInterval)
}

func initialMetricsInterval() (time.Duration, error) {
	return apmconfig.ParseDurationEnv(envMetricsInterval, "s", defaultMetricsInterval)
}

func initialMaxTransactionQueueSize() (int, error) {
	value := os.Getenv(envMaxQueueSize)
	if value == "" {
		return defaultMaxTransactionQueueSize, nil
	}
	size, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse %s", envMaxQueueSize)
	}
	return size, nil
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
	source := rand.NewSource(time.Now().Unix())
	return NewRatioSampler(ratio, source), nil
}

func initialSanitizedFieldNamesRegexp() (*regexp.Regexp, error) {
	value := os.Getenv(envSanitizeFieldNames)
	if value == "" {
		return defaultSanitizedFieldNames, nil
	}
	re, err := regexp.Compile(fmt.Sprintf("(?i:%s)", value))
	if err != nil {
		_, err = regexp.Compile(value)
		return nil, errors.Wrapf(err, "invalid %s value", envSanitizeFieldNames)
	}
	return re, nil
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
	return apmconfig.ParseDurationEnv(envSpanFramesMinDuration, "", defaultSpanFramesMinDuration)
}

func initialActive() (bool, error) {
	return apmconfig.ParseBoolEnv(envActive, true)
}

func initialDistributedTracing() (bool, error) {
	return apmconfig.ParseBoolEnv(envDistributedTracing, false)
}
