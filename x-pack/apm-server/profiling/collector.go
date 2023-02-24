// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/golang-lru/simplelru"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/common"
	"github.com/elastic/apm-server/x-pack/apm-server/profiling/libpf"
)

var (
	// metrics
	indexerDocs                 = monitoring.Default.NewRegistry("apm-server.profiling.indexer.document")
	counterEventsTotal          = monitoring.NewInt(indexerDocs, "events.total.count")
	counterEventsFailure        = monitoring.NewInt(indexerDocs, "events.failure.count")
	counterStacktracesTotal     = monitoring.NewInt(indexerDocs, "stacktraces.total.count")
	counterStacktracesDuplicate = monitoring.NewInt(indexerDocs, "stacktraces.duplicate.count")
	counterStacktracesFailure   = monitoring.NewInt(indexerDocs, "stacktraces.failure.count")
	counterStackframesTotal     = monitoring.NewInt(indexerDocs, "stackframes.total.count")
	counterStackframesDuplicate = monitoring.NewInt(indexerDocs, "stackframes.duplicate.count")
	counterStackframesFailure   = monitoring.NewInt(indexerDocs, "stackframes.failure.count")
	counterExecutablesTotal     = monitoring.NewInt(indexerDocs, "executables.total.count")
	counterExecutablesFailure   = monitoring.NewInt(indexerDocs, "executables.failure.count")

	counterFatalErr = monitoring.NewInt(nil, "apm-server.profiling.unrecoverable_error.count")

	// gRPC error returned to the clients
	errCustomer = status.Error(codes.Internal, "failed to process request")
)

const (
	actionCreate = "create"
	actionUpdate = "update"

	sourceFileCacheSize = 128 * 1024
	// ES error string indicating a duplicate document by _id
	docIDAlreadyExists = "version_conflict_engine_exception"
)

// ElasticCollector is an implementation of the gRPC server handling the data
// sent by Host-Agent.
type ElasticCollector struct {
	// See https://github.com/grpc/grpc-go/issues/3669 for why this struct is embedded.
	UnimplementedCollectionAgentServer

	logger         *logp.Logger
	indexer        esutil.BulkIndexer
	metricsIndexer esutil.BulkIndexer
	indexes        [common.MaxEventsIndexes]string

	sourceFilesLock sync.Mutex
	sourceFiles     *simplelru.LRU

	clusterID string
}

// NewCollector returns a new ElasticCollector uses indexer for storing stack trace data in
// Elasticsearch, and metricsIndexer for storing host agent metrics. Separate indexers are
// used to allow for host agent metrics to be sent to a separate monitoring cluster.
func NewCollector(
	indexer esutil.BulkIndexer,
	metricsIndexer esutil.BulkIndexer,
	esClusterID string,
	logger *logp.Logger,
) *ElasticCollector {
	sourceFiles, err := simplelru.NewLRU(sourceFileCacheSize, nil)
	if err != nil {
		log.Fatalf("Failed to create source file LRU: %v", err)
	}

	c := &ElasticCollector{
		logger:         logger,
		indexer:        indexer,
		metricsIndexer: metricsIndexer,
		sourceFiles:    sourceFiles,
		clusterID:      esClusterID,
	}

	// Precalculate index names to minimise per-TraceEvent overhead.
	for i := range c.indexes {
		c.indexes[i] = fmt.Sprintf("%s-%dpow%02d", common.EventsIndexPrefix,
			common.SamplingFactor, i+1)
	}
	return c
}

// AddCountsForTraces implements the RPC to send stacktrace data: stacktrace hashes and counts.
func (e *ElasticCollector) AddCountsForTraces(ctx context.Context,
	req *AddCountsForTracesRequest) (*emptypb.Empty, error) {
	traceEvents, err := mapToStackTraceEvents(ctx, req)
	if err != nil {
		e.logger.With(logp.Error(err)).Error("Error mapping host-agent traces to Elastic stacktraces")
		return nil, errCustomer
	}
	counterEventsTotal.Add(int64(len(traceEvents)))

	// Store every event as-is into the full events index.
	e.logger.Infof("adding %d trace events", len(traceEvents))
	for i := range traceEvents {
		if err := e.indexStacktrace(ctx, &traceEvents[i], common.AllEventsIndex); err != nil {
			return nil, errCustomer
		}
	}

	// Each event has a probability of p=1/5=0.2 to go from one index into the next downsampled
	// index. Since we aggregate identical stacktrace events by timestamp when reported and stored,
	// we have a 'Count' value for each. To be statistically correct, we have to apply p=0.2 to
	// each single stacktrace event independently and not just to the aggregate. We can do so by
	// looping over 'Count' and apply p=0.2 on every iteration to generate a new 'Count' value for
	// the next downsampled index.
	// We only store aggregates with 'Count' > 0. If 'Count' becomes 0, we are done and can
	// continue with the next stacktrace event.
	for i := range traceEvents {
		for _, index := range e.indexes {
			count := uint16(0)
			for j := uint16(0); j < traceEvents[i].Count; j++ {
				// samplingRatio is the probability p=0.2 for an event to be copied into the next
				// downsampled index.
				if rand.Float64() < common.SamplingRatio { //nolint:gosec
					count++
				}
			}
			if count == 0 {
				// We are done with this event, process the next one.
				break
			}

			// Store the event with its new downsampled count in the downsampled index.
			traceEvents[i].Count = count

			if err := e.indexStacktrace(ctx, &traceEvents[i], index); err != nil {
				e.logger.With(logp.Error(err)).Error("Elasticsearch indexing error")
				return nil, errCustomer
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (e *ElasticCollector) indexStacktrace(ctx context.Context, traceEvent *StackTraceEvent,
	indexName string) (err error) {
	var encodedTraceEvent bytes.Buffer
	_ = json.NewEncoder(&encodedTraceEvent).Encode(*traceEvent)

	return e.indexer.Add(ctx, esutil.BulkIndexerItem{
		Index:  indexName,
		Action: actionCreate,
		Body:   bytes.NewReader(encodedTraceEvent.Bytes()),
		OnFailure: func(
			_ context.Context,
			_ esutil.BulkIndexerItem,
			resp esutil.BulkIndexerResponseItem,
			err error,
		) {
			counterEventsFailure.Inc()
			e.logger.With(
				logp.Error(err),
				logp.String("index", indexName),
				logp.String("error_type", resp.Error.Type),
			).Errorf("failed to index stacktrace event: %s", resp.Error.Reason)
		},
	})
}

// StackTraceEvent represents a stacktrace event serializable into ES.
// The json field names need to be case-sensitively equal to the fields defined
// in the schema mapping.
type StackTraceEvent struct {
	common.EcsVersion
	ProjectID uint32 `json:"service.name"`
	TimeStamp uint32 `json:"@timestamp"`
	HostID    uint64 `json:"host.id"`
	// 128-bit hash in binary form
	StackTraceID  string `json:"Stacktrace.id"`
	PodName       string `json:"orchestrator.resource.name,omitempty"`
	ContainerName string `json:"container.name,omitempty"`
	ThreadName    string `json:"process.thread.name"`
	Count         uint16 `json:"Stacktrace.count"`

	// Host metadata
	Tags []string `json:"tags,omitempty"`
	// HostIP is the list of network cards IPs, mapped to an Elasticsearch "ip" data type field
	HostIP []string `json:"host.ip,omitempty"`
	// HostIPString is the list of network cards IPs, mapped to an Elasticsearch "keyword" data type
	HostIPString string `json:"host.ipstring,omitempty"`
	HostName     string `json:"host.name,omitempty"`
	OSKernel     string `json:"os.kernel,omitempty"`
	AgentVersion string `json:"agent.version,omitempty"`
}

// StackTrace represents a stacktrace serializable into the stacktraces index.
// DocID should be the base64-encoded Stacktrace ID.
type StackTrace struct {
	common.EcsVersion
	FrameIDs string `json:"Stacktrace.frame.ids"`
	Types    string `json:"Stacktrace.frame.types"`
}

// StackFrame represents a stacktrace serializable into the stackframes index.
// DocID should be the base64-encoded FileID+Address (24 bytes).
type StackFrame struct {
	common.EcsVersion
	FileName       string `json:"Stackframe.file.name,omitempty"`
	FunctionName   string `json:"Stackframe.function.name,omitempty"`
	LineNumber     int32  `json:"Stackframe.line.number,omitempty"`
	FunctionOffset int32  `json:"Stackframe.function.offset,omitempty"`
}

// mapToStackTraceEvents maps Prodfiler stacktraces to Elastic documents.
func mapToStackTraceEvents(ctx context.Context,
	req *AddCountsForTracesRequest) ([]StackTraceEvent, error) {
	traces, err := CollectTracesAndCounts(req)
	if err != nil {
		return nil, err
	}

	ts := req.GetTimestamp()
	projectID := GetProjectID(ctx)
	hostID := GetHostID(ctx)
	kernelVersion := GetKernelVersion(ctx)
	hostName := GetHostname(ctx)
	agentVersion := GetRevision(ctx)

	tags := strings.Split(GetTags(ctx), ";")
	if len(tags) == 1 && tags[0] == "" {
		// prevent storing 'tags'
		tags = nil
	}

	ipAddress := GetIPAddress(ctx)
	ipAddresses := []string{ipAddress}
	if ipAddress == "" {
		// prevent storing 'host.ip'
		ipAddresses = nil
	}

	traceEvents := make([]StackTraceEvent, 0, len(traces))
	for i := range traces {
		traceEvents = append(traceEvents,
			StackTraceEvent{
				ProjectID:     projectID,
				TimeStamp:     ts,
				HostID:        hostID,
				StackTraceID:  common.EncodeStackTraceID(traces[i].Hash),
				PodName:       traces[i].PodName,
				ContainerName: traces[i].ContainerName,
				ThreadName:    traces[i].Comm,
				Count:         traces[i].Count,
				Tags:          tags,
				HostIP:        ipAddresses,
				HostIPString:  ipAddress,
				HostName:      hostName,
				OSKernel:      kernelVersion,
				AgentVersion:  agentVersion,
			})
	}

	return traceEvents, nil
}

// SaveHostInfo is needed too otherwise host-agent will not start properly
func (*ElasticCollector) SaveHostInfo(context.Context, *HostInfo) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Script written in Painless that will both create a new document (if DocID does not exist),
// and update timestamp of an existing document. Named parameters are used to improve performance
// re: script compilation (since the script does not change across executions, it can be compiled
// once and cached).
const exeMetadataUpsertScript = `
if (ctx.op == 'create') {
    ctx._source['@timestamp']            = params.timestamp;
    ctx._source['Executable.build.id']   = params.buildid;
    ctx._source['Executable.file.name']  = params.filename;
    ctx._source['ecs.version']           = params.ecsversion;
} else {
    if (ctx._source['@timestamp'] == params.timestamp) {
        ctx.op = 'noop'
    } else {
        ctx._source['@timestamp'] = params.timestamp
    }
}
`

type ExeMetadataScript struct {
	Source string            `json:"source"`
	Params ExeMetadataParams `json:"params"`
}

type ExeMetadataParams struct {
	LastSeen   uint32 `json:"timestamp"`
	BuildID    string `json:"buildid"`
	FileName   string `json:"filename"`
	EcsVersion string `json:"ecsversion"`
}

// ExeMetadata represents executable metadata serializable into the executables index.
// DocID should be the base64-encoded FileID.
type ExeMetadata struct {
	// ScriptedUpsert needs to be 'true' for the script to execute regardless of the
	// document existing or not.
	ScriptedUpsert bool              `json:"scripted_upsert"`
	Script         ExeMetadataScript `json:"script"`
	// This needs to exist for document creation to succeed (if document does not exist),
	// but can be empty as the script implements both document creation and updating.
	Upsert struct{} `json:"upsert"`
}

func (e *ElasticCollector) AddExecutableMetadata(ctx context.Context,
	in *AddExecutableMetadataRequest) (*empty.Empty, error) {
	hiFileIDs := in.GetHiFileIDs()
	loFileIDs := in.GetLoFileIDs()

	numHiFileIDs := len(hiFileIDs)
	numLoFileIDs := len(loFileIDs)

	if numHiFileIDs == 0 {
		e.logger.Debug("AddExecutableMetadata request with no entries")
		return &empty.Empty{}, nil
	}

	// Sanity check. Should never happen unless the HA is broken.
	if numHiFileIDs != numLoFileIDs {
		e.logger.Errorf(
			"mismatch in number of file IDAs (%d) file IDBs (%d)",
			numHiFileIDs, numLoFileIDs,
		)
		counterFatalErr.Inc()
		return nil, errCustomer
	}

	counterExecutablesTotal.Add(int64(numHiFileIDs))

	filenames := in.GetFilenames()
	buildIDs := in.GetBuildIDs()

	lastSeen := common.GetStartOfWeekFromTime(time.Now())

	for i := 0; i < numHiFileIDs; i++ {
		fileID := libpf.NewFileID(hiFileIDs[i], loFileIDs[i])

		// DocID is the base64-encoded FileID.
		docID := common.EncodeFileID(fileID)

		exeMetadata := ExeMetadata{
			ScriptedUpsert: true,
			Script: ExeMetadataScript{
				Source: exeMetadataUpsertScript,
				Params: ExeMetadataParams{
					LastSeen:   lastSeen,
					BuildID:    buildIDs[i],
					FileName:   filenames[i],
					EcsVersion: common.EcsVersionString,
				},
			},
		}
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(exeMetadata)

		err := multiplexCurrentNextIndicesWrite(ctx, e, &esutil.BulkIndexerItem{
			Index:      common.ExecutablesIndex,
			Action:     actionUpdate,
			DocumentID: docID,
			OnFailure: func(
				_ context.Context,
				_ esutil.BulkIndexerItem,
				resp esutil.BulkIndexerResponseItem,
				err error,
			) {
				counterExecutablesFailure.Inc()
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
				).Errorf("failed to index executable metadata: %s", resp.Error.Reason)
			},
		}, buf.Bytes())
		if err != nil {
			e.logger.With(logp.Error(err)).Error("Elasticsearch indexing error")
			return nil, errCustomer
		}
	}

	return &emptypb.Empty{}, nil
}

// Heartbeat is needed too otherwise host-agent will not start properly
func (*ElasticCollector) Heartbeat(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// ReportHostMetadata is needed too otherwise host-agent will not start properly
func (*ElasticCollector) ReportHostMetadata(context.Context,
	*HostMetadata) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (e *ElasticCollector) SetFramesForTraces(ctx context.Context,
	req *SetFramesForTracesRequest) (*empty.Empty, error) {

	traces, err := CollectTracesAndFrames(req)
	if err != nil {
		counterFatalErr.Inc()
		return nil, err
	}
	counterStacktracesTotal.Add(int64(len(traces)))

	for _, trace := range traces {
		// We use the base64-encoded trace hash as the document ID. This seems to be an
		// appropriate way to do K/V lookups with ES.
		docID := common.EncodeStackTraceID(trace.Hash)

		toIndex := StackTrace{
			FrameIDs: common.EncodeFrameIDs(trace.Files, trace.Linenos),
			Types:    common.EncodeFrameTypes(trace.FrameTypes),
		}

		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(toIndex)

		err = multiplexCurrentNextIndicesWrite(ctx, e, &esutil.BulkIndexerItem{
			Index:      common.StackTraceIndex,
			Action:     actionCreate,
			DocumentID: docID,
			OnFailure: func(
				_ context.Context,
				_ esutil.BulkIndexerItem,
				resp esutil.BulkIndexerResponseItem,
				_ error,
			) {
				if resp.Error.Type == docIDAlreadyExists {
					// Error is expected here, as we tried to "create" an existing document.
					// We increment the metric to understand the origin-to-duplicate ratio.
					counterStacktracesDuplicate.Inc()
					return
				}
				counterStacktracesFailure.Inc()
			},
		}, body.Bytes())

		if err != nil {
			e.logger.With(logp.Error(err),
				logp.String("grpc_method", "SetFramesForTraces"),
			).Error("Elasticsearch indexing error: %v", err)
			return nil, errCustomer
		}
	}

	return &emptypb.Empty{}, nil
}

func (e *ElasticCollector) AddFrameMetadata(ctx context.Context, in *AddFrameMetadataRequest) (
	*empty.Empty, error) {
	hiFileIDs := in.GetHiFileIDs()
	loFileIDs := in.GetLoFileIDs()
	hiSourceIDs := in.GetHiSourceIDs()
	loSourceIDs := in.GetLoSourceIDs()
	addressOrLines := in.GetAddressOrLines()
	lineNumbers := in.GetLineNumbers()
	functionNames := in.GetFunctionNames()
	functionOffsets := in.GetFunctionOffsets()
	filenames := in.GetFilenames()

	arraySize := len(hiFileIDs)
	if arraySize == 0 {
		e.logger.Debug("AddFrameMetadata request with no entries")
		return &empty.Empty{}, nil
	}

	// Sanity check. Should never happen unless the HA is broken or client is malicious.
	if arraySize != len(loFileIDs) ||
		arraySize != len(hiSourceIDs) ||
		arraySize != len(loSourceIDs) ||
		arraySize != len(addressOrLines) ||
		arraySize != len(lineNumbers) ||
		arraySize != len(functionNames) ||
		arraySize != len(functionOffsets) ||
		arraySize != len(filenames) {
		counterFatalErr.Inc()
		e.logger.Errorf("mismatch in array sizes (%d)", arraySize)
		return nil, errCustomer
	}
	counterStackframesTotal.Add(int64(arraySize))

	for i := 0; i < arraySize; i++ {
		fileID := libpf.NewFileID(hiFileIDs[i], loFileIDs[i])

		if fileID.IsZero() {
			e.logger.Warn("Attempting to report metadata for invalid FileID 0." +
				" This is likely a mistake and will be discarded.",
			)
			continue
		}

		sourceFileID := libpf.NewFileID(hiSourceIDs[i], loSourceIDs[i])
		filename := filenames[i]
		e.sourceFilesLock.Lock()
		if filename == "" {
			if v, ok := e.sourceFiles.Get(sourceFileID); ok {
				filename = v.(string)
			}
		} else {
			e.sourceFiles.Add(sourceFileID, filename)
		}
		e.sourceFilesLock.Unlock()

		frameMetadata := StackFrame{
			LineNumber:     int32(lineNumbers[i]),
			FunctionName:   functionNames[i],
			FunctionOffset: int32(functionOffsets[i]),
			FileName:       filename,
		}

		docID := common.EncodeFrameID(fileID, addressOrLines[i])

		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(frameMetadata)

		err := multiplexCurrentNextIndicesWrite(ctx, e, &esutil.BulkIndexerItem{
			Index:      common.StackFrameIndex,
			Action:     actionCreate,
			DocumentID: docID,
			OnFailure: func(
				_ context.Context,
				_ esutil.BulkIndexerItem,
				resp esutil.BulkIndexerResponseItem,
				_ error,
			) {
				if resp.Error.Type == docIDAlreadyExists {
					// Error is expected here, as we tried to "create" an existing document.
					// We increment the metric to understand the origin-to-duplicate ratio.
					counterStackframesDuplicate.Inc()
					return
				}
				counterStackframesFailure.Inc()
			},
		}, buf.Bytes())
		if err != nil {
			e.logger.With(logp.Error(err),
				logp.String("grpc_method", "AddFrameMetadata"),
			).Error("Elasticsearch indexing error")
			return nil, errCustomer
		}
	}

	return &empty.Empty{}, nil
}

func (e *ElasticCollector) AddFallbackSymbols(ctx context.Context,
	in *AddFallbackSymbolsRequest) (*empty.Empty, error) {
	hiFileIDs := in.GetHiFileIDs()
	loFileIDs := in.GetLoFileIDs()
	symbols := in.GetSymbols()
	addressOrLines := in.GetAddressOrLines()

	arraySize := len(hiFileIDs)
	if arraySize == 0 {
		e.logger.Debug("AddFallbackSymbols request with no entries")
		return &empty.Empty{}, nil
	}

	// Sanity check. Should never happen unless the HA is broken or client is malicious.
	if arraySize != len(loFileIDs) ||
		arraySize != len(addressOrLines) ||
		arraySize != len(symbols) {
		e.logger.Errorf("mismatch in array sizes (%d)", arraySize)
		counterFatalErr.Inc()
		return nil, errCustomer
	}
	counterStackframesTotal.Add(int64(arraySize))

	for i := 0; i < arraySize; i++ {
		fileID := libpf.NewFileID(hiFileIDs[i], loFileIDs[i])

		if fileID.IsZero() {
			e.logger.Warn("" +
				"Attempting to report metadata for invalid FileID 0." +
				" This is likely a mistake and will be discarded.",
			)
			continue
		}

		frameMetadata := StackFrame{
			FunctionName: symbols[i],
		}

		docID := common.EncodeFrameID(fileID, addressOrLines[i])

		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(frameMetadata)

		err := multiplexCurrentNextIndicesWrite(ctx, e, &esutil.BulkIndexerItem{
			Index: common.StackFrameIndex,
			// Use 'create' instead of 'index' to not overwrite an existing document,
			// possibly containing a fully symbolized frame.
			Action:     actionCreate,
			DocumentID: docID,
			OnFailure: func(
				_ context.Context,
				_ esutil.BulkIndexerItem,
				resp esutil.BulkIndexerResponseItem,
				err error,
			) {
				if resp.Error.Type == docIDAlreadyExists {
					// Error is expected here, as we tried to "create" an existing document.
					// We increment the metric to understand the origin-to-duplicate ratio.
					counterStackframesDuplicate.Inc()
					return
				}
				counterStackframesFailure.Inc()
			},
		}, buf.Bytes())
		if err != nil {
			e.logger.With(logp.Error(err),
				logp.String("grpc_method", "AddFallbackSymbols"),
			).Error("Elasticsearch indexing error")
			return nil, errCustomer
		}
	}

	return &empty.Empty{}, nil
}

//go:embed metrics.json
var metricsDefFS embed.FS

type metricDef struct {
	Description string `json:"description"`
	MetricType  string `json:"type"`
	Name        string `json:"name"`
	FieldName   string `json:"field"`
	ID          uint32 `json:"id"`
	Obsolete    bool   `json:"obsolete"`
}

var fieldNames []string
var metricTypes []string

func init() {
	input, err := metricsDefFS.ReadFile("metrics.json")
	if err != nil {
		log.Fatalf("Failed to read from embedded metrics.json: %v", err)
	}

	var metricDefs []metricDef
	if err = json.Unmarshal(input, &metricDefs); err != nil {
		log.Fatalf("Failed to unmarshal from embedded metrics.json: %v", err)
	}

	maxID := uint32(0)
	for _, m := range metricDefs {
		// Plausibility check, we don't expect having that many metrics.
		if m.ID > 1000 {
			log.Fatalf("Unexpected high metric ID %d (needs manual adjustment)", m.ID)
		}
		if m.ID > maxID {
			maxID = m.ID
		}
	}

	fieldNames = make([]string, maxID+1)
	metricTypes = make([]string, maxID+1)

	for _, m := range metricDefs {
		if m.Obsolete {
			continue
		}
		fieldNames[m.ID] = m.FieldName
		metricTypes[m.ID] = m.MetricType
	}
}

func (e *ElasticCollector) AddMetrics(ctx context.Context, in *Metrics) (*empty.Empty, error) {
	tsmetrics := in.GetTsMetrics()
	ProjectID := GetProjectID(ctx)
	HostID := GetHostID(ctx)

	makeBody := func(metric *TsMetric) *bytes.Reader {
		var body bytes.Buffer

		body.WriteString(fmt.Sprintf(
			"{\"project.id\":%d,\"host.id\":%d,\"@timestamp\":%d,"+
				"\"ecs.version\":%q",
			ProjectID, HostID, metric.Timestamp, common.EcsVersionString))
		if e.clusterID != "" {
			body.WriteString(fmt.Sprintf(",\"Elasticsearch.cluster.id\":%q", e.clusterID))
		}
		for i, metricID := range metric.IDs {
			if int(metricID) >= len(metricTypes) {
				// Protect against panic on HA / collector version mismatch.
				// Do not log as this may happen often.
				continue
			}
			metricValue := metric.Values[i]
			metricType := metricTypes[metricID]
			fieldName := fieldNames[metricID]

			if metricValue == 0 && metricType == "counter" {
				// HA accidentally sends 0 counter values. Here we ignore them.
				// This check can be removed once the issue is fixed in the host agent.
				continue
			}

			if fieldName == "" {
				continue
			}

			body.WriteString(
				fmt.Sprintf(",%q:%d", fieldName, metricValue))
		}

		body.WriteString("}")
		return bytes.NewReader(body.Bytes())
	}

	for _, metric := range tsmetrics {
		if len(metric.IDs) != len(metric.Values) {
			e.logger.Errorf(
				"Ignoring inconsistent metrics (ids: %d != values: %d)",
				len(metric.IDs), len(metric.Values),
			)
			continue
		}
		err := e.metricsIndexer.Add(ctx, esutil.BulkIndexerItem{
			Index:  common.MetricsIndex,
			Action: actionCreate,
			Body:   makeBody(metric),
			OnFailure: func(
				_ context.Context,
				_ esutil.BulkIndexerItem,
				resp esutil.BulkIndexerResponseItem,
				err error,
			) {
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
					logp.String("grpc_method", "AddMetrics"),
				).Error("failed to index host metrics")
			},
		})
		if err != nil {
			return nil, errCustomer
		}
	}

	return &empty.Empty{}, nil
}

// multiplexCurrentNextIndicesWrite ingests twice the same item for 2 separate indices
// to achieve a sliding window ingestion mechanism.
// These indices will be managed by the custom ILM strategy implemented in ilm.go.
func multiplexCurrentNextIndicesWrite(ctx context.Context, e *ElasticCollector,
	item *esutil.BulkIndexerItem, body []byte) error {
	copied := *item
	copied.Index = nextIndex(item.Index)

	item.Body = bytes.NewReader(body)
	copied.Body = bytes.NewReader(body)

	if err := e.indexer.Add(ctx, *item); err != nil {
		return err
	}
	return e.indexer.Add(ctx, copied)
}
