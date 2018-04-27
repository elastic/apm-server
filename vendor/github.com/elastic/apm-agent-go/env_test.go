package elasticapm_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestTracerFlushIntervalEnv(t *testing.T) {
	t.Run("suffix", func(t *testing.T) {
		testTracerFlushIntervalEnv(t, "1s", time.Second)
	})
	t.Run("no_suffix", func(t *testing.T) {
		testTracerFlushIntervalEnv(t, "1", time.Second)
	})
}

func TestTracerFlushIntervalEnvInvalid(t *testing.T) {
	os.Setenv("ELASTIC_APM_FLUSH_INTERVAL", "aeon")
	defer os.Unsetenv("ELASTIC_APM_FLUSH_INTERVAL")

	_, err := elasticapm.NewTracer("tracer_testing", "")
	assert.EqualError(t, err, "failed to parse ELASTIC_APM_FLUSH_INTERVAL: time: invalid duration aeon")
}

func testTracerFlushIntervalEnv(t *testing.T, envValue string, expectedInterval time.Duration) {
	os.Setenv("ELASTIC_APM_FLUSH_INTERVAL", envValue)
	defer os.Unsetenv("ELASTIC_APM_FLUSH_INTERVAL")

	tracer, err := elasticapm.NewTracer("tracer_testing", "")
	require.NoError(t, err)
	defer tracer.Close()
	tracer.Transport = transporttest.Discard

	before := time.Now()
	tracer.StartTransaction("name", "type").Done(-1)
	assert.Equal(t, elasticapm.TracerStats{TransactionsSent: 0}, tracer.Stats())
	for tracer.Stats().TransactionsSent == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.WithinDuration(t, before.Add(expectedInterval), time.Now(), 100*time.Millisecond)
}

func TestTracerTransactionRateEnv(t *testing.T) {
	t.Run("0.5", func(t *testing.T) {
		testTracerTransactionRateEnv(t, "0.5", 0.5)
	})
	t.Run("0.75", func(t *testing.T) {
		testTracerTransactionRateEnv(t, "0.75", 0.75)
	})
	t.Run("1.0", func(t *testing.T) {
		testTracerTransactionRateEnv(t, "1.0", 1.0)
	})
}

func TestTracerTransactionRateEnvInvalid(t *testing.T) {
	os.Setenv("ELASTIC_APM_TRANSACTION_SAMPLE_RATE", "2.0")
	defer os.Unsetenv("ELASTIC_APM_TRANSACTION_SAMPLE_RATE")

	_, err := elasticapm.NewTracer("tracer_testing", "")
	assert.EqualError(t, err, "invalid ELASTIC_APM_TRANSACTION_SAMPLE_RATE value 2.0: out of range [0,1.0]")
}

func testTracerTransactionRateEnv(t *testing.T, envValue string, ratio float64) {
	os.Setenv("ELASTIC_APM_TRANSACTION_SAMPLE_RATE", envValue)
	defer os.Unsetenv("ELASTIC_APM_TRANSACTION_SAMPLE_RATE")

	tracer, err := elasticapm.NewTracer("tracer_testing", "")
	require.NoError(t, err)
	defer tracer.Close()
	tracer.Transport = transporttest.Discard

	const N = 10000
	var sampled int
	for i := 0; i < N; i++ {
		tx := tracer.StartTransaction("name", "type")
		if tx.Sampled() {
			sampled++
		}
		tx.Done(-1)
	}
	assert.InDelta(t, N*ratio, sampled, N*0.02) // allow 2% error
}

func TestTracerServiceNameEnvSanitizationSpecified(t *testing.T) {
	testTracerServiceNameSanitization(
		t, "TestTracerServiceNameEnvSanitizationSpecified",
		"foo_bar", "ELASTIC_APM_SERVICE_NAME=foo!bar",
	)
}

func TestTracerServiceNameEnvSanitizationExecutableName(t *testing.T) {
	testTracerServiceNameSanitization(
		t, "TestTracerServiceNameEnvSanitizationExecutableName",
		"apm-agent-go_test", // .test -> _test
	)
}

func testTracerServiceNameSanitization(t *testing.T, testName, sanitizedServiceName string, env ...string) {
	if os.Getenv("_INSIDE_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
		cmd.Env = append(cmd.Env, "_INSIDE_TEST=1")
		cmd.Env = append(cmd.Env, env...)
		err := cmd.Run()
		assert.NoError(t, err)
		return
	}

	tracer, err := elasticapm.NewTracer("", "")
	require.NoError(t, err)
	defer tracer.Close()

	var called bool
	tracer.Transport = transporttest.CallbackTransport{
		Transactions: func(_ context.Context, payload *model.TransactionsPayload) error {
			assert.Equal(t, sanitizedServiceName, payload.Service.Name)
			called = true
			return nil
		},
	}

	tx := tracer.StartTransaction("name", "type")
	tx.Done(-1)
	tracer.Flush(nil)
	assert.True(t, called)
}
