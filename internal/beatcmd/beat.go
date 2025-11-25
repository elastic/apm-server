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

package beatcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"go.elastic.co/apm/module/apmotel/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/api"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/management"
	"github.com/elastic/beats/v7/libbeat/monitoring/report"
	"github.com/elastic/beats/v7/libbeat/monitoring/report/log"
	"github.com/elastic/beats/v7/libbeat/pprof"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/file"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/monitoring/report/buffer"
	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/elastic/elastic-agent-libs/service"
	"github.com/elastic/elastic-agent-system-metrics/metric/system/host"
	metricreport "github.com/elastic/elastic-agent-system-metrics/report"
	sysinfo "github.com/elastic/go-sysinfo"
	"github.com/elastic/go-sysinfo/types"

	"github.com/elastic/apm-server/internal/version"
)

const (
	defaultMonitoringUsername = "apm_system"
)

// Beat provides the runnable and configurable instance of a beat.
type Beat struct {
	beat.Beat

	Config *Config

	rawConfig *config.C
	newRunner NewRunnerFunc

	tracerProvider trace.TracerProvider
	metricReader   *sdkmetric.ManualReader
	meterProvider  *sdkmetric.MeterProvider
	metricGatherer *apmotel.Gatherer
}

// BeatParams holds parameters for NewBeat.
type BeatParams struct {
	// NewRunner holds a NewRunnerFunc for creating a Runner.
	//
	// If (Fleet) management is enabled, NewRunner may be called multiple
	// times, whenever configuration is reloaded. Otherwise, NewRunner will
	// be called once with the initial, static, configuration.
	NewRunner NewRunnerFunc

	// ElasticLicensed indicates whether this build of APM Server
	// is licensed with the Elastic License v2.
	ElasticLicensed bool

	// Logger holds the logger used by the runner
	Logger *logp.Logger
}

// NewBeat creates a new Beat.
func NewBeat(args BeatParams) (*Beat, error) {
	cfg, rawConfig, keystore, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	beatName := cfg.Name
	if beatName == "" {
		beatName = hostname
	}

	ephemeralID, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral ID: %w", err)
	}

	exporter, err := apmotel.NewGatherer()
	if err != nil {
		return nil, err
	}

	metricReader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithReader(metricReader),
	)
	otel.SetMeterProvider(meterProvider)

	b := &Beat{
		Beat: beat.Beat{
			Info: beat.Info{
				Beat:            "apm-server",
				ElasticLicensed: args.ElasticLicensed,
				IndexPrefix:     "apm-server",
				Version:         version.VersionWithQualifier(),
				Name:            beatName,
				Hostname:        hostname,
				StartTime:       time.Now(),
				EphemeralID:     ephemeralID,
				Logger:          args.Logger,
			},
			Keystore:   keystore,
			Config:     &beat.BeatConfig{Output: cfg.Output},
			BeatConfig: cfg.APMServer,
			Registry:   reload.NewRegistry(),
			Monitoring: beat.NewMonitoring(),
		},
		Config:         cfg,
		newRunner:      args.NewRunner,
		rawConfig:      rawConfig,
		metricReader:   metricReader,
		meterProvider:  meterProvider,
		metricGatherer: &exporter,
	}

	if err := b.init(); err != nil {
		return nil, err
	}
	return b, nil
}

// init initializes logging, config management, GOMAXPROCS, and GC percent.
func (b *Beat) init() error {
	if err := configureLogging(b.Config); err != nil {
		return fmt.Errorf("failed to configure logging: %w", err)
	}
	if b.Info.Logger == nil {
		b.Info.Logger = logp.NewLogger("")
	}

	// log paths values to help with troubleshooting
	b.Info.Logger.Infof("%s", paths.Paths.String())

	// Load the unique ID and "first start" info from meta.json.
	metaPath := paths.Resolve(paths.Data, "meta.json")
	if err := b.loadMeta(metaPath); err != nil {
		return err
	}
	b.Info.Logger.Infof("Beat ID: %v", b.Info.ID)

	// Initialize central config manager.
	manager, err := management.NewManager(b.Config.Management, b.Registry, b.Info.Logger)
	if err != nil {
		return err
	}
	b.Manager = manager

	if maxProcs := b.Config.MaxProcs; maxProcs > 0 {
		b.Info.Logger.Infof("Set max procs limit: %v", maxProcs)
		runtime.GOMAXPROCS(maxProcs)
	}
	if gcPercent := b.Config.GCPercent; gcPercent > 0 {
		b.Info.Logger.Infof("Set gc percentage to: %v", gcPercent)
		debug.SetGCPercent(gcPercent)
	}
	return nil
}

func (b *Beat) loadMeta(metaPath string) error {
	type meta struct {
		UUID       uuid.UUID `json:"uuid"`
		FirstStart time.Time `json:"first_start"`
	}

	b.Info.Logger.Named("beat").Debugf("Beat metadata path: %v", metaPath)
	f, err := openRegular(metaPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("meta file failed to open: %w", err)
	}
	if err == nil {
		var m meta
		if err := json.NewDecoder(f).Decode(&m); err != nil && err != io.EOF {
			f.Close()
			return fmt.Errorf("Beat meta file reading error: %w", err)
		}
		f.Close()
		b.Info.FirstStart = m.FirstStart
		b.Info.ID = m.UUID
	}

	rewrite := false
	if b.Info.FirstStart.IsZero() {
		b.Info.FirstStart = b.Info.StartTime
		rewrite = true
	}
	if b.Info.ID == uuid.Nil {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		b.Info.ID = id
		rewrite = true
	}
	if !rewrite {
		return nil
	}

	// meta.json does not exist, or the contents are invalid: write a new file.
	//
	// Write a temporary file, and then atomically move it into place in case
	// of errors occurring half way through.

	encodedMeta, err := json.Marshal(meta{
		UUID:       b.Info.ID,
		FirstStart: b.Info.FirstStart,
	})
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	tempFile := metaPath + ".new"
	if err := os.WriteFile(tempFile, encodedMeta, 0600); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// move temporary file into final location
	return file.SafeFileRotate(metaPath, tempFile)
}

func openRegular(filename string) (*os.File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return f, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	if !info.Mode().IsRegular() {
		f.Close()
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a directory", filename)
		}
		return nil, fmt.Errorf("%s is not a regular file", filename)
	}

	return f, nil
}

func (b *Beat) Run(ctx context.Context) error {
	defer b.Info.Logger.Sync()
	defer func() {
		if r := recover(); r != nil {
			b.Info.Logger.Fatalw("exiting due to panic",
				"panic", r,
				zap.Stack("stack"),
			)
		}
	}()
	defer b.Info.Logger.Infof("%s stopped.", b.Info.Beat)

	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	defer g.Wait() // ensure all goroutines exit before Run returns
	defer cancel()

	// Try to acquire exclusive lock on data path to prevent another beat instance
	// sharing same data path.
	locker := newLocker(b)
	if err := locker.lock(); err != nil {
		return err
	}
	defer locker.unlock()

	service.BeforeRun()
	defer service.Cleanup()

	b.registerMetrics()

	// Start the libbeat API server for serving stats, state, etc.
	var apiServer *api.Server
	if b.Config.HTTP.Enabled() {
		var err error
		apiServer, err = api.NewWithDefaultRoutes(b.Info.Logger, b.Config.HTTP,
			b.Monitoring.InfoRegistry(),
			b.Monitoring.StateRegistry(),
			b.Monitoring.StatsRegistry(),
			b.Monitoring.InputsRegistry(),
		)
		if err != nil {
			return fmt.Errorf("could not start the HTTP server for the API: %w", err)
		}
		apiServer.Start()
		defer apiServer.Stop()
		if b.Config.HTTPPprof.IsEnabled() {
			pprof.SetRuntimeProfilingParameters(b.Config.HTTPPprof)
			if err := pprof.HttpAttach(b.Config.HTTPPprof, apiServer); err != nil {
				return fmt.Errorf("failed to attach http handlers for pprof: %w", err)
			}
		}
	}

	monitoringReporter, err := b.setupMonitoring()
	if err != nil {
		return err
	}
	if monitoringReporter != nil {
		defer monitoringReporter.Stop()
	}

	if b.Config.MetricLogging != nil && b.Config.MetricLogging.Enabled() {
		reporter, err := log.MakeReporter(b.Info, b.Config.MetricLogging)
		if err != nil {
			return err
		}
		defer reporter.Stop()
	}

	// If enabled, collect metrics into a ring buffer.
	//
	// TODO(axw) confirm that this is used by Elastic Agent. If not, remove it?
	// This is not mentioned in our docs.
	if b.Config.HTTP.Enabled() && monitoring.IsBufferEnabled(b.Config.BufferConfig) {
		buffReporter, err := buffer.MakeReporter(b.Config.BufferConfig)
		if err != nil {
			return err
		}
		defer buffReporter.Stop()
		if err := apiServer.AttachHandler("/buffer", buffReporter); err != nil {
			return err
		}
	}

	slogger := slog.New(zapslog.NewHandler(b.Info.Logger.Core()))
	g.Go(func() error {
		return adjustMemlimit(ctx, 30*time.Second, slogger)
	})

	logSystemInfo(b.Info)

	statsRegistry := b.Monitoring.StatsRegistry()

	if err := metricreport.SetupMetricsOptions(metricreport.MetricOptions{
		Name:           b.Info.Beat,
		Version:        b.Info.Version,
		EphemeralID:    b.Info.EphemeralID.String(),
		Logger:         b.Info.Logger.Named("metrics"),
		SystemMetrics:  statsRegistry.GetOrCreateRegistry("system"),
		ProcessMetrics: statsRegistry.GetOrCreateRegistry("beat"),
	}); err != nil {
		return err
	}

	if b.Manager.Enabled() {
		reloader, err := NewReloader(b.Info, b.Registry, b.newRunner, b.meterProvider, b.metricGatherer, b.tracerProvider, b.Monitoring)
		if err != nil {
			return err
		}
		g.Go(func() error { return reloader.Run(ctx) })

		b.Manager.SetStopCallback(cancel)
		if err := b.Manager.Start(); err != nil {
			return fmt.Errorf("failed to start manager: %w", err)
		}
		defer b.Manager.Stop()
	} else {
		if !b.Config.Output.IsSet() {
			return errors.New("no output defined, please define one under the output section")
		}
		runner, err := b.newRunner(RunnerParams{
			Config:          b.rawConfig,
			Info:            b.Info,
			Logger:          b.Info.Logger,
			TracerProvider:  b.tracerProvider,
			MeterProvider:   b.meterProvider,
			MetricsGatherer: b.metricGatherer,
			BeatMonitoring:  b.Monitoring,
		})
		if err != nil {
			return err
		}
		g.Go(func() error { return runner.Run(ctx) })
	}
	b.Info.Logger.Infof("%s started.", b.Info.Beat)
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// registerMetrics registers metrics with the internal monitoring API. This data
// is then exposed through the HTTP monitoring endpoint (e.g. /info and /state)
// and/or pushed to Elasticsearch through the x-pack monitoring feature.
func (b *Beat) registerMetrics() {
	b.registerInfoMetrics()
	b.registerStateMetrics()
	b.registerStatsMetrics()
}

func (b *Beat) registerInfoMetrics() {
	infoRegistry := b.Monitoring.InfoRegistry()
	monitoring.NewString(infoRegistry, "version").Set(b.Info.Version)
	monitoring.NewString(infoRegistry, "beat").Set(b.Info.Beat)
	monitoring.NewString(infoRegistry, "name").Set(b.Info.Name)
	monitoring.NewString(infoRegistry, "hostname").Set(b.Info.Hostname)
	monitoring.NewString(infoRegistry, "uuid").Set(b.Info.ID.String())
	monitoring.NewString(infoRegistry, "ephemeral_id").Set(b.Info.EphemeralID.String())
	monitoring.NewString(infoRegistry, "binary_arch").Set(runtime.GOARCH)
	monitoring.NewString(infoRegistry, "build_commit").Set(version.CommitHash())
	monitoring.NewTimestamp(infoRegistry, "build_time").Set(version.CommitTime())
	monitoring.NewBool(infoRegistry, "elastic_licensed").Set(b.Info.ElasticLicensed)

	// Add user metadata data asynchronously (on Windows the lookup can take up to 60s).
	go func() {
		if u, err := user.Current(); err != nil {
			// This usually happens if the user UID does not exist in /etc/passwd. It might be the case on K8S
			// if the user set securityContext.runAsUser to an arbitrary value.
			monitoring.NewString(infoRegistry, "uid").Set(strconv.Itoa(os.Getuid()))
			monitoring.NewString(infoRegistry, "gid").Set(strconv.Itoa(os.Getgid()))
		} else {
			monitoring.NewString(infoRegistry, "username").Set(u.Username)
			monitoring.NewString(infoRegistry, "uid").Set(u.Uid)
			monitoring.NewString(infoRegistry, "gid").Set(u.Gid)
		}
	}()
}

func (b *Beat) registerStateMetrics() {
	stateRegistry := b.Monitoring.StateRegistry()

	// state.service
	serviceRegistry := stateRegistry.GetOrCreateRegistry("service")
	monitoring.NewString(serviceRegistry, "version").Set(b.Info.Version)
	monitoring.NewString(serviceRegistry, "name").Set(b.Info.Beat)
	monitoring.NewString(serviceRegistry, "id").Set(b.Info.ID.String())

	// state.beat
	beatRegistry := stateRegistry.GetOrCreateRegistry("beat")
	monitoring.NewString(beatRegistry, "name").Set(b.Info.Name)

	// state.host
	monitoring.NewFunc(stateRegistry, "host", host.ReportInfo("" /* don't use FQDN */), monitoring.Report)

	// state.management
	managementRegistry := stateRegistry.GetOrCreateRegistry("management")
	monitoring.NewBool(managementRegistry, "enabled").Set(b.Manager.Enabled())
}

func (b *Beat) registerStatsMetrics() {
	statsRegistry := b.Monitoring.StatsRegistry()
	libbeatRegistry := statsRegistry.GetOrCreateRegistry("libbeat")
	monitoring.NewFunc(libbeatRegistry, "output", func(_ monitoring.Mode, v monitoring.Visitor) {
		var rm metricdata.ResourceMetrics
		if err := b.metricReader.Collect(context.Background(), &rm); err != nil {
			return
		}
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		for _, sm := range rm.ScopeMetrics {
			switch {
			case sm.Scope.Name == "github.com/elastic/go-docappender":
				monitoring.ReportString(v, "type", "elasticsearch")
				addDocappenderLibbeatOutputMetrics(context.Background(), v, sm)
			}
		}
	})
	monitoring.NewFunc(libbeatRegistry, "pipeline", func(_ monitoring.Mode, v monitoring.Visitor) {
		var rm metricdata.ResourceMetrics
		if err := b.metricReader.Collect(context.Background(), &rm); err != nil {
			return
		}
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		for _, sm := range rm.ScopeMetrics {
			switch {
			case sm.Scope.Name == "github.com/elastic/go-docappender":
				addDocappenderLibbeatPipelineMetrics(context.Background(), v, sm)
			}
		}
	})
	monitoring.NewFunc(statsRegistry, "output.elasticsearch", func(_ monitoring.Mode, v monitoring.Visitor) {
		var rm metricdata.ResourceMetrics
		if err := b.metricReader.Collect(context.Background(), &rm); err != nil {
			return
		}
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		for _, sm := range rm.ScopeMetrics {
			switch {
			case sm.Scope.Name == "github.com/elastic/go-docappender":
				addDocappenderOutputElasticsearchMetrics(context.Background(), v, sm)
			}
		}
	})
	monitoring.NewFunc(statsRegistry, "apm-server", func(_ monitoring.Mode, v monitoring.Visitor) {
		var rm metricdata.ResourceMetrics
		if err := b.metricReader.Collect(context.Background(), &rm); err != nil {
			return
		}
		v.OnRegistryStart()
		defer v.OnRegistryFinished()

		// first collect all apm-server metrics
		beatsMetrics := make(map[string]any)
		for _, sm := range rm.ScopeMetrics {
			switch {
			case strings.HasPrefix(sm.Scope.Name, "github.com/elastic/apm-server"):
				// All simple scalar metrics that begin with the name "apm-server."
				// in github.com/elastic/apm-server/... scopes are mapped directly.
				addAPMServerMetricsToMap(beatsMetrics, sm.Metrics)
			}
		}

		// register all metrics once
		// this prevents metrics with the same prefix in the name
		// from different scoped meters from overwriting each other
		reportOnKey(v, beatsMetrics)
	})
}

// getScalarInt64 returns a single-value, dimensionless
// gauge or counter integer value, or (0, false) if the
// data does not match these constraints.
func getScalarInt64(data metricdata.Aggregation) (int64, bool) {
	switch data := data.(type) {
	case metricdata.Sum[int64]:
		if len(data.DataPoints) != 1 || data.DataPoints[0].Attributes.Len() != 0 {
			break
		}
		return data.DataPoints[0].Value, true
	case metricdata.Gauge[int64]:
		if len(data.DataPoints) != 1 || data.DataPoints[0].Attributes.Len() != 0 {
			break
		}
		return data.DataPoints[0].Value, true
	}
	return 0, false
}

// addAPMServerMetricsToMap adds simple scalar metrics with the "apm-server." prefix
// to the map.
func addAPMServerMetricsToMap(beatsMetrics map[string]any, metrics []metricdata.Metrics) {
	for _, m := range metrics {
		if suffix, ok := strings.CutPrefix(m.Name, "apm-server."); ok {
			if value, ok := getScalarInt64(m.Data); ok {
				current := beatsMetrics
				suffixSlice := strings.Split(suffix, ".")
				for i := 0; i < len(suffixSlice)-1; i++ {
					k := suffixSlice[i]
					if _, ok := current[k]; !ok {
						current[k] = make(map[string]any)
					}
					if currentmap, ok := current[k].(map[string]any); ok {
						current = currentmap
					}
				}
				current[suffixSlice[len(suffixSlice)-1]] = value
			}
		}
	}
}

func reportOnKey(v monitoring.Visitor, m map[string]any) {
	for key, value := range m {
		if valueMap, ok := value.(map[string]any); ok {
			v.OnRegistryStart()
			v.OnKey(key)
			reportOnKey(v, valueMap)
			v.OnRegistryFinished()
		}
		if valueMetric, ok := value.(int64); ok {
			monitoring.ReportInt(v, key, valueMetric)
		}
	}
}

// Adapt go-docappender's OTel metrics to beats stack monitoring metrics,
// with a mixture of libbeat-specific and apm-server specific metric names.
func addDocappenderLibbeatOutputMetrics(ctx context.Context, v monitoring.Visitor, sm metricdata.ScopeMetrics) {
	var writeBytes int64

	v.OnRegistryStart()
	v.OnKey("events")
	for _, m := range sm.Metrics {
		switch m.Name {
		case "elasticsearch.events.processed":
			var acked, toomany, failed int64
			data, _ := m.Data.(metricdata.Sum[int64])
			for _, dp := range data.DataPoints {
				status, ok := dp.Attributes.Value(attribute.Key("status"))
				if !ok {
					continue
				}
				switch status.AsString() {
				case "Success":
					acked += dp.Value
				case "TooMany":
					toomany += dp.Value
					fallthrough
				default:
					failed += dp.Value
				}
			}
			monitoring.ReportInt(v, "acked", acked)
			monitoring.ReportInt(v, "failed", failed)
			monitoring.ReportInt(v, "toomany", toomany)
		case "elasticsearch.events.count":
			if value, ok := getScalarInt64(m.Data); ok {
				monitoring.ReportInt(v, "total", value)
			}
		case "elasticsearch.events.queued":
			if value, ok := getScalarInt64(m.Data); ok {
				monitoring.ReportInt(v, "active", value)
			}
		case "elasticsearch.flushed.bytes":
			if value, ok := getScalarInt64(m.Data); ok {
				writeBytes = value
			}
		case "elasticsearch.bulk_requests.count":
			if value, ok := getScalarInt64(m.Data); ok {
				monitoring.ReportInt(v, "batches", value)
			}
		}
	}
	v.OnRegistryFinished()

	if writeBytes > 0 {
		v.OnRegistryStart()
		v.OnKey("write")
		monitoring.ReportInt(v, "bytes", writeBytes)
		v.OnRegistryFinished()
	}
}

func addDocappenderLibbeatPipelineMetrics(ctx context.Context, v monitoring.Visitor, sm metricdata.ScopeMetrics) {
	v.OnRegistryStart()
	defer v.OnRegistryFinished()
	v.OnKey("events")

	for _, m := range sm.Metrics {
		switch m.Name {
		case "elasticsearch.events.count":
			if value, ok := getScalarInt64(m.Data); ok {
				monitoring.ReportInt(v, "total", value)
			}
		}
	}
}

// Add non-libbeat Elasticsearch output metrics under "output.elasticsearch".
func addDocappenderOutputElasticsearchMetrics(ctx context.Context, v monitoring.Visitor, sm metricdata.ScopeMetrics) {
	var bulkRequestsCount, bulkRequestsAvailable int64
	var indexersCreated, indexersDestroyed int64
	for _, m := range sm.Metrics {
		switch m.Name {
		case "elasticsearch.bulk_requests.count":
			if value, ok := getScalarInt64(m.Data); ok {
				bulkRequestsCount = value
			}
		case "elasticsearch.bulk_requests.available":
			if value, ok := getScalarInt64(m.Data); ok {
				bulkRequestsAvailable = value
			}
		case "elasticsearch.indexer.created":
			if value, ok := getScalarInt64(m.Data); ok {
				indexersCreated = value
			}
		case "elasticsearch.indexer.destroyed":
			if value, ok := getScalarInt64(m.Data); ok {
				indexersDestroyed = value
			}
		}
	}

	v.OnRegistryStart()
	v.OnKey("bulk_requests")
	monitoring.ReportInt(v, "completed", bulkRequestsCount)
	monitoring.ReportInt(v, "available", bulkRequestsAvailable)
	v.OnRegistryFinished()

	v.OnRegistryStart()
	v.OnKey("indexers")
	monitoring.ReportInt(v, "created", indexersCreated)
	monitoring.ReportInt(v, "destroyed", indexersDestroyed)
	monitoring.ReportInt(v, "active", indexersCreated-indexersDestroyed+1)
	v.OnRegistryFinished()
}

func (b *Beat) setupMonitoring() (report.Reporter, error) {
	monitoringCfg := b.Config.Monitoring

	monitoringClusterUUID, err := monitoring.GetClusterUUID(monitoringCfg)
	if err != nil {
		return nil, err
	}

	// Expose monitoring.cluster_uuid in state API
	if monitoringClusterUUID != "" {
		stateRegistry := b.Monitoring.StateRegistry()
		monitoringRegistry := stateRegistry.GetOrCreateRegistry("monitoring")
		clusterUUIDRegVar := monitoring.NewString(monitoringRegistry, "cluster_uuid")
		clusterUUIDRegVar.Set(monitoringClusterUUID)
	}

	if monitoring.IsEnabled(monitoringCfg) {
		err := monitoring.OverrideWithCloudSettings(monitoringCfg)
		if err != nil {
			return nil, err
		}
		settings := report.Settings{
			DefaultUsername: defaultMonitoringUsername,
			ClusterUUID:     monitoringClusterUUID,
		}
		reporter, err := report.New(b.Info, b.Monitoring, settings, monitoringCfg, b.Config.Output)
		if err != nil {
			return nil, err
		}
		return reporter, nil
	}

	return nil, nil
}

// logSystemInfo logs information about this system for situational awareness
// in debugging. This information includes data about the beat, build, go
// runtime, host, and process. If any of the data is not available it will be
// omitted.
func logSystemInfo(info beat.Info) {
	defer info.Logger.Recover("An unexpected error occurred while collecting " +
		"information about the system.")
	log := info.Logger.Named("beat").With(logp.Namespace("system_info"))

	// Beat
	beat := mapstr.M{
		"type": info.Beat,
		"uuid": info.ID,
		"path": mapstr.M{
			"config": paths.Resolve(paths.Config, ""),
			"data":   paths.Resolve(paths.Data, ""),
			"home":   paths.Resolve(paths.Home, ""),
			"logs":   paths.Resolve(paths.Logs, ""),
		},
	}
	log.Infow("Beat info", "beat", beat)

	// Build
	build := mapstr.M{
		"commit":  version.CommitHash(),
		"time":    version.CommitTime(),
		"version": info.Version,
	}
	log.Infow("Build info", "build", build)

	// Go Runtime
	log.Infow("Go runtime info", "go", sysinfo.Go())

	// Host
	if host, err := sysinfo.Host(); err == nil {
		log.Infow("Host info", "host", host.Info())
	}

	// Process
	if self, err := sysinfo.Self(); err == nil {
		process := mapstr.M{}

		if info, err := self.Info(); err == nil {
			process["name"] = info.Name
			process["pid"] = info.PID
			process["ppid"] = info.PPID
			process["cwd"] = info.CWD
			process["exe"] = info.Exe
			process["start_time"] = info.StartTime
		}

		if proc, ok := self.(types.Seccomp); ok {
			if seccomp, err := proc.Seccomp(); err == nil {
				process["seccomp"] = seccomp
			}
		}

		if proc, ok := self.(types.Capabilities); ok {
			if caps, err := proc.Capabilities(); err == nil {
				process["capabilities"] = caps
			}
		}

		if len(process) > 0 {
			log.Infow("Process info", "process", process)
		}
	}
}
