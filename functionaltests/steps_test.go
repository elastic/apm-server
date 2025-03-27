package functionaltests

import (
	"context"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/gen"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/apm-server/functionaltests/internal/terraform"
)

func runTestSteps(t *testing.T, namespace string, startVersion ecclient.StackVersion, steps []testStep) {
	if namespace == "" {
		namespace = "default"
	}

	start := time.Now()
	ctx := context.Background()
	tf := initTerraformRunner(t)

	t.Logf("------ cluster setup %s ------", startVersion.String())
	esCfg := createCluster(t, ctx, tf, *target, startVersion.String())
	esc := createESClient(t, esCfg)
	kbc := createKibanaClient(t, ctx, esc, esCfg)
	g := createAPMGenerator(t, ctx, esc, esCfg)
	t.Logf("time elapsed: %s", time.Since(start))

	env := testStepEnv{
		namespace: namespace,
		version:   startVersion,
		tf:        tf,
		gen:       g,
		kbc:       kbc,
		esc:       esc,
	}

	docCounts, err := getDocsCountPerDS(t, ctx, esc)
	require.NoError(t, err)

	currentRes := testStepResult{DocCounts: docCounts}
	for _, step := range steps {
		currentRes = step.Step(t, ctx, &env, currentRes)
		t.Logf("time elapsed: %s", time.Since(start))
	}
}

type testStepResult struct {
	DocCounts esclient.DataStreamsDocCount
}

type testStepEnv struct {
	namespace string
	version   ecclient.StackVersion
	tf        *terraform.Runner
	gen       *gen.Generator
	kbc       *kbclient.Client
	esc       *esclient.Client
}

type testStep interface {
	Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult
}

type ingestionStep struct {
	checkDataStream checkDataStreamWant
}

var _ testStep = ingestionStep{}

func (i ingestionStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ ingestion ------")
	err := e.gen.RunBlockingWait(ctx, e.kbc, e.version.String())
	require.NoError(t, err)

	t.Log("------ ingestion check ------")
	t.Log("check number of documents after ingestion")
	docCounts, err := getDocsCountPerDS(t, ctx, e.esc)
	require.NoError(t, err)
	assertDocCount(t, docCounts, previousRes.DocCounts,
		expectedIngestForASingleRun(e.namespace),
		aggregationDataStreams(e.namespace))

	t.Log("check data streams after ingestion")
	dss, err := e.esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, i.checkDataStream, dss)

	return testStepResult{DocCounts: docCounts}
}

type upgradeStep struct {
	newVersion      ecclient.StackVersion
	checkDataStream checkDataStreamWant
}

var _ testStep = upgradeStep{}

func (u upgradeStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Logf("------ upgrade from %s to %s ------", e.version.String(), u.newVersion.String())
	upgradeCluster(t, ctx, e.tf, *target, u.newVersion.String())
	e.version = u.newVersion

	t.Log("------ upgrade check ------")
	t.Log("check number of documents across upgrade")
	docCounts, err := getDocsCountPerDS(t, ctx, e.esc)
	require.NoError(t, err)
	assertDocCount(t, docCounts, previousRes.DocCounts,
		emptyIngestForASingleRun(e.namespace),
		aggregationDataStreams(e.namespace))

	t.Log("check data streams after upgrade")
	dss, err := e.esc.GetDataStream(ctx, "*apm*")
	require.NoError(t, err)
	assertDataStreams(t, u.checkDataStream, dss)

	return testStepResult{DocCounts: docCounts}
}

type checkErrorLogsStep struct {
	apmErrorLogsIgnored []types.Query
}

var _ testStep = checkErrorLogsStep{}

func (c checkErrorLogsStep) Step(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
	t.Log("------ check ES and APM error logs ------")
	t.Log("checking ES error logs")
	resp, err := e.esc.GetESErrorLogs(ctx)
	require.NoError(t, err)
	asserts.ZeroESLogs(t, *resp)

	t.Log("checking APM error logs")
	resp, err = e.esc.GetAPMErrorLogs(ctx, c.apmErrorLogsIgnored)
	require.NoError(t, err)
	asserts.ZeroAPMLogs(t, *resp)

	return previousRes
}
