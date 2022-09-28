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
	"strconv"
	"strings"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/golang-lru/simplelru"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-server/internal/elasticsearch"
)

var (
	// metrics
	indexerDocs               = monitoring.Default.NewRegistry("apm-server.profiling.indexer.document")
	counterEventsTotal        = monitoring.NewInt(indexerDocs, "events.total.count")
	counterEventsFailure      = monitoring.NewInt(indexerDocs, "events.failure.count")
	counterStacktracesTotal   = monitoring.NewInt(indexerDocs, "stacktraces.total.count")
	counterStacktracesFailure = monitoring.NewInt(indexerDocs, "stacktraces.failure.count")
	counterStackframesTotal   = monitoring.NewInt(indexerDocs, "stackframes.total.count")
	counterStackframesFailure = monitoring.NewInt(indexerDocs, "stackframes.failure.count")
	counterExecutablesTotal   = monitoring.NewInt(indexerDocs, "executables.total.count")
	counterExecutablesFailure = monitoring.NewInt(indexerDocs, "executables.failure.count")

	counterFatalErr = monitoring.NewInt(nil, "apm-server.profiling.unrecoverable_error.count")

	// gRPC error returned to the clients
	errCustomer = status.Error(codes.Internal, "failed to process request")
)

const (
	actionIndex  = "index"
	actionCreate = "create"

	sourceFileCacheSize = 128 * 1024
)

// ElasticCollector is an implementation of the gRPC server handling the data
// sent by Host-Agent.
type ElasticCollector struct {
	// See https://github.com/grpc/grpc-go/issues/3669 for why this struct is embedded.
	UnimplementedCollectionAgentServer

	logger         *logp.Logger
	indexer        elasticsearch.BulkIndexer
	metricsIndexer elasticsearch.BulkIndexer
	indexes        [MaxEventsIndexes]string

	sourceFilesLock sync.Mutex
	sourceFiles     *simplelru.LRU
}

// NewCollector returns a new ElasticCollector uses indexer for storing stack trace data in
// Elasticsearch, and metricsIndexer for storing host agent metrics. Separate indexers are
// used to allow for host agent metrics to be sent to a separate monitoring cluster.
func NewCollector(indexer elasticsearch.BulkIndexer, metricsIndexer elasticsearch.BulkIndexer,
	esClusterName string, logger *logp.Logger, ) *ElasticCollector {
	sourceFiles, err := simplelru.NewLRU(sourceFileCacheSize, nil)
	if err != nil {
		log.Fatalf("Failed to create source file LRU: %v", err)
	}

	c := &ElasticCollector{
		logger:         logger,
		indexer:        indexer,
		metricsIndexer: metricsIndexer,
		sourceFiles:    sourceFiles,
	}

	// Precalculate index names to minimise per-TraceEvent overhead.
	for i := range c.indexes {
		c.indexes[i] = fmt.Sprintf("%s-%dpow%02d", EventsIndexPrefix, SamplingFactor, i+1)
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
		if err := e.indexStacktrace(ctx, &traceEvents[i], AllEventsIndex); err != nil {
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
				if rand.Float64() < SamplingRatio { //nolint:gosec
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

	return e.indexer.Add(ctx, elasticsearch.BulkIndexerItem{
		Index:  indexName,
		Action: actionCreate,
		Body:   bytes.NewReader(encodedTraceEvent.Bytes()),
		OnFailure: func(
			_ context.Context,
			_ elasticsearch.BulkIndexerItem,
			resp elasticsearch.BulkIndexerResponseItem,
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
	ECSVersion ecsVersion `json:"ecs.version"`
	ProjectID  uint32     `json:"service.name"`
	TimeStamp  uint32     `json:"@timestamp"`
	HostID     uint64     `json:"host.id"`
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
	ECSVersion ecsVersion `json:"ecs.version"`
	FrameIDs   string     `json:"Stacktrace.frame.ids"`
	Types      string     `json:"Stacktrace.frame.types"`
	LastSeen   uint32     `json:"@timestamp"`
}

// StackFrame represents a stacktrace serializable into the stackframes index.
// DocID should be the base64-encoded FileID+Address (24 bytes).
type StackFrame struct {
	ECSVersion     ecsVersion `json:"ecs.version"`
	FileName       string     `json:"Stackframe.file.name,omitempty"`
	FunctionName   string     `json:"Stackframe.function.name,omitempty"`
	LineNumber     int32      `json:"Stackframe.line.number,omitempty"`
	FunctionOffset int32      `json:"Stackframe.function.offset,omitempty"`
	SourceType     int16      `json:"Stackframe.source.type,omitempty"`
}

// ExecutableMetadata represents executable metadata serializable into the executables index.
// DocID should be the base64-encoded FileID.
type ExecutableMetadata struct {
	ECSVersion ecsVersion `json:"ecs.version"`
	BuildID    string     `json:"Executable.build.id"`
	FileName   string     `json:"Executable.file.name"`
	LastSeen   uint32     `json:"@timestamp"`
}

const ecsVersionString = "1.12.0"

type ecsVersion struct{}

func (e ecsVersion) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(ecsVersionString)), nil
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
				StackTraceID:  EncodeStackTraceID(traces[i].Hash),
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

func (e *ElasticCollector) AddExecutableMetadata(ctx context.Context,
	in *AddExecutableMetadataRequest) (*empty.Empty, error) {
	hiFileIDs := in.GetHiFileIDs()
	loFileIDs := in.GetLoFileIDs()

	lastSeen := GetStartOfWeek()

	numHiFileIDs := len(hiFileIDs)
	numLoFileIDs := len(loFileIDs)

	// Sanity check. Should never happen unless the HA is broken.
	if numHiFileIDs != numLoFileIDs {
		e.logger.Errorf(
			"mismatch in number of file IDAs (%d) file IDBs (%d)",
			numHiFileIDs, numLoFileIDs,
		)
		counterFatalErr.Inc()
		return nil, errCustomer
	}

	if numHiFileIDs == 0 {
		e.logger.Debug("AddExecutableMetadata request with no entries")
		return &empty.Empty{}, nil
	}
	counterExecutablesTotal.Add(int64(numHiFileIDs))

	filenames := in.GetFilenames()
	buildIDs := in.GetBuildIDs()

	for i := 0; i < numHiFileIDs; i++ {
		fileID := NewFileID(hiFileIDs[i], loFileIDs[i])

		// DocID is the base64-encoded FileID.
		docID := EncodeFileID(fileID)

		exeMetadata := ExecutableMetadata{
			LastSeen: lastSeen,
			BuildID:  buildIDs[i],
			FileName: filenames[i],
		}
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(exeMetadata)

		err := e.indexer.Add(ctx, elasticsearch.BulkIndexerItem{
			Index:      ExecutablesIndex,
			Action:     actionIndex,
			DocumentID: docID,
			Body:       bytes.NewReader(buf.Bytes()),
			OnFailure: func(
				_ context.Context,
				_ elasticsearch.BulkIndexerItem,
				resp elasticsearch.BulkIndexerResponseItem,
				err error,
			) {
				counterExecutablesFailure.Inc()
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
				).Errorf("failed to index executable metadata: %s", resp.Error.Reason)
			},
		})
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
	lastSeen := GetStartOfWeek()

	traces, err := CollectTracesAndFrames(req)
	if err != nil {
		counterFatalErr.Inc()
		return nil, err
	}
	counterStacktracesTotal.Add(int64(len(traces)))

	for _, trace := range traces {
		// We use the base64-encoded trace hash as the document ID. This seems to be an
		// appropriate way to do K/V lookups with ES.
		docID := EncodeStackTraceID(trace.Hash)

		toIndex := StackTrace{
			FrameIDs: EncodeFrameIDs(trace.Files, trace.Linenos),
			Types:    EncodeFrameTypes(trace.FrameTypes),
			LastSeen: lastSeen,
		}

		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(toIndex)

		err = e.indexer.Add(ctx, elasticsearch.BulkIndexerItem{
			Index:      StackTraceIndex,
			Action:     actionIndex,
			DocumentID: docID,
			Body:       bytes.NewReader(body.Bytes()),
			OnFailure: func(
				_ context.Context,
				_ elasticsearch.BulkIndexerItem,
				resp elasticsearch.BulkIndexerResponseItem,
				err error,
			) {
				counterStacktracesFailure.Inc()
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
				).Errorf("failed to index stacktrace metadata: %s", resp.Error.Reason)
			},
		})

		if err != nil {
			e.logger.With(logp.Error(err)).Error("Elasticsearch indexing error")
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
	types := in.GetTypes()
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
		arraySize != len(types) ||
		arraySize != len(filenames) {
		counterFatalErr.Inc()
		e.logger.Errorf("mismatch in array sizes (%d)", arraySize)
		return nil, errCustomer
	}
	counterStackframesTotal.Add(int64(arraySize))

	for i := 0; i < arraySize; i++ {
		fileID := NewFileID(hiFileIDs[i], loFileIDs[i])

		if fileID.IsZero() {
			e.logger.Warn("Attempting to report metadata for invalid FileID 0." +
				" This is likely a mistake and will be discarded.",
			)
			continue
		}

		sourceFileID := NewFileID(hiSourceIDs[i], loSourceIDs[i])
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
			SourceType:     int16(types[i]),
		}

		docID := EncodeFrameID(fileID, addressOrLines[i])

		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(frameMetadata)

		// If the filename is empty, we don't want to replace an existing record, because
		// it may contain a filename already. That's why we use "create" in this case.
		action := actionIndex
		if frameMetadata.FileName == "" {
			action = actionCreate
		}

		err := e.indexer.Add(ctx, elasticsearch.BulkIndexerItem{
			Index:      StackFrameIndex,
			Action:     action,
			DocumentID: docID,
			Body:       bytes.NewReader(buf.Bytes()),
			OnFailure: func(
				_ context.Context,
				item elasticsearch.BulkIndexerItem,
				resp elasticsearch.BulkIndexerResponseItem,
				err error,
			) {
				if item.Action == actionCreate {
					// Error is expected here, as we tried to "create" an existing document
					return
				}
				counterStackframesFailure.Inc()
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
				).Errorf("failed to index stackframe metadata: %s", resp.Error.Reason)
			},
		})
		if err != nil {
			e.logger.With(logp.Error(err)).Error("Elasticsearch indexing error")
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
		fileID := NewFileID(hiFileIDs[i], loFileIDs[i])

		if fileID.IsZero() {
			e.logger.Warn("" +
				"Attempting to report metadata for invalid FileID 0." +
				" This is likely a mistake and will be discarded.",
			)
			continue
		}

		frameMetadata := StackFrame{
			FunctionName: symbols[i],
			SourceType:   SourceTypeC,
		}

		docID := EncodeFrameID(fileID, addressOrLines[i])

		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(frameMetadata)

		err := e.indexer.Add(ctx, elasticsearch.BulkIndexerItem{
			Index: StackFrameIndex,
			// Use 'create' instead of 'index' to not overwrite an existing document,
			// possibly containing a fully symbolized frame.
			Action:     actionCreate,
			DocumentID: docID,
			Body:       bytes.NewReader(buf.Bytes()),
			OnFailure: func(
				_ context.Context,
				_ elasticsearch.BulkIndexerItem,
				resp elasticsearch.BulkIndexerResponseItem,
				err error,
			) {
				counterStackframesFailure.Inc()
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
				).Error("failed to index stackframe metadata: %s", resp.Error.Reason)
			},
		})
		if err != nil {
			e.logger.With(logp.Error(err)).Error("Elasticsearch indexing error")
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
				"\"ecs.version\":\"%s\"",
			ProjectID, HostID, metric.Timestamp, ecsVersionString))

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
		err := e.metricsIndexer.Add(ctx, elasticsearch.BulkIndexerItem{
			Index:  MetricsIndex,
			Action: actionCreate,
			Body:   makeBody(metric),
			OnFailure: func(
				_ context.Context,
				_ elasticsearch.BulkIndexerItem,
				resp elasticsearch.BulkIndexerResponseItem,
				err error,
			) {
				e.logger.With(
					logp.Error(err),
					logp.String("error_type", resp.Error.Type),
				).Error("failed to index host metrics")
			},
		})
		if err != nil {
			return nil, errCustomer
		}
	}

	return &empty.Empty{}, nil
}
