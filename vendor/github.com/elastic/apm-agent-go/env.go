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
)

const (
	envFlushInterval         = "ELASTIC_APM_FLUSH_INTERVAL"
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

	defaultFlushInterval           = 10 * time.Second
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
	value := os.Getenv(envFlushInterval)
	if value == "" {
		return defaultFlushInterval, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		// We allow the value to have no suffix, in which case
		// we assume seconds, to be compatible with configuration
		// for other Elastic APM agents.
		var err2 error
		d, err2 = time.ParseDuration(value + "s")
		if err2 == nil {
			err = nil
		}
	}
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse %s", envFlushInterval)
	}
	return d, nil
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
	value := os.Getenv(envSpanFramesMinDuration)
	if value == "" {
		return defaultSpanFramesMinDuration, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse %s", envSpanFramesMinDuration)
	}
	return d, nil
}

func initialActive() (bool, error) {
	value := os.Getenv(envActive)
	if value == "" {
		return true, nil
	}
	active, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse %s", envActive)
	}
	return active, nil
}
