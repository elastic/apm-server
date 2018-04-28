package elasticapm

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

const (
	envFlushInterval         = "ELASTIC_APM_FLUSH_INTERVAL"
	envMaxQueueSize          = "ELASTIC_APM_MAX_QUEUE_SIZE"
	envMaxSpans              = "ELASTIC_APM_TRANSACTION_MAX_SPANS"
	envTransactionSampleRate = "ELASTIC_APM_TRANSACTION_SAMPLE_RATE"

	defaultFlushInterval           = 10 * time.Second
	defaultMaxTransactionQueueSize = 500
	defaultMaxSpans                = 500
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
