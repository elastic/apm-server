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
	"os"
	"os/user"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"

	"github.com/elastic/beats/v7/libbeat/api"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/instrumentation"
	"github.com/elastic/beats/v7/libbeat/licenser"
	"github.com/elastic/beats/v7/libbeat/management"
	"github.com/elastic/beats/v7/libbeat/monitoring/report"
	"github.com/elastic/beats/v7/libbeat/monitoring/report/log"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/file"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/configure"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/monitoring/report/buffer"
	"github.com/elastic/elastic-agent-libs/paths"
	svc "github.com/elastic/elastic-agent-libs/service"
	libversion "github.com/elastic/elastic-agent-libs/version"
	"github.com/elastic/elastic-agent-system-metrics/metric/system/host"
	metricreport "github.com/elastic/elastic-agent-system-metrics/report"
	sysinfo "github.com/elastic/go-sysinfo"
	"github.com/elastic/go-sysinfo/types"

	"github.com/elastic/apm-server/internal/version"
)

const (
	defaultMonitoringUsername = "apm_system"
)

var (
	libbeatMetricsRegistry = monitoring.Default.GetRegistry("libbeat")
)

// Beat provides the runnable and configurable instance of a beat.
type Beat struct {
	beat.Beat

	Config *Config

	rawConfig *config.C
	create    beat.Creator
}

// BeatParams holds parameters for NewBeat.
type BeatParams struct {
	// Create holds a beat.Creator for creating an instance of beat.Beater.
	Create beat.Creator

	// ElasticLicensed indicates whether this build of APM Server
	// is licensed with the Elastic License v2.
	ElasticLicensed bool
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

	b := &Beat{
		Beat: beat.Beat{
			Info: beat.Info{
				Beat:            "apm-server",
				ElasticLicensed: args.ElasticLicensed,
				IndexPrefix:     "apm-server",
				Version:         version.Version,
				Name:            hostname,
				Hostname:        hostname,
				StartTime:       time.Now(),
				EphemeralID:     metricreport.EphemeralID(),
			},
			Keystore:   keystore,
			Config:     &beat.BeatConfig{Output: cfg.Output},
			BeatConfig: cfg.APMServer,
		},
		Config:    cfg,
		create:    args.Create,
		rawConfig: rawConfig,
	}

	if err := b.init(); err != nil {
		return nil, err
	}
	return b, nil
}

// init initializes logging, tracing ("instrumentation"), monitoring, config
// management, and GOMAXPROCS.
func (b *Beat) init() error {
	if err := configure.Logging(b.Info.Beat, b.Config.Logging); err != nil {
		return fmt.Errorf("error initializing logging: %w", err)
	}
	// log paths values to help with troubleshooting
	logp.Info(paths.Paths.String())

	// instrumentation.New expects a config object with "instrumentation"
	// as a child, so create a new config with instrumentation added.
	instrumentation, err := instrumentation.New(b.rawConfig, b.Info.Beat, b.Info.Version)
	if err != nil {
		return err
	}
	b.Instrumentation = instrumentation

	// Load the unique ID and "first start" info from meta.json.
	metaPath := paths.Resolve(paths.Data, "meta.json")
	if err := b.loadMeta(metaPath); err != nil {
		return err
	}
	logp.Info("Beat ID: %v", b.Info.ID)

	// Initialize central config manager.
	b.Manager, err = management.Factory(b.Config.Management)(b.Config.Management, reload.Register, b.Beat.Info.ID)
	if err != nil {
		return err
	}

	if maxProcs := b.Config.MaxProcs; maxProcs > 0 {
		logp.Info("Set max procs limit: %v", maxProcs)
		runtime.GOMAXPROCS(maxProcs)
	}
	if gcPercent := b.Config.GCPercent; gcPercent > 0 {
		logp.Info("Set gc percentage to: %v", gcPercent)
		debug.SetGCPercent(gcPercent)
	}
	return nil
}

func (b *Beat) loadMeta(metaPath string) error {
	type meta struct {
		UUID       uuid.UUID `json:"uuid"`
		FirstStart time.Time `json:"first_start"`
	}

	logp.Debug("beat", "Beat metadata path: %v", metaPath)
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

// create and return the beater, this method also initializes all needed items,
// including template registering, publisher, xpack monitoring
func (b *Beat) createBeater(beatCreator beat.Creator) (beat.Beater, error) {
	logSystemInfo(b.Info)

	cleanup, err := b.registerElasticsearchVersionCheck()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	cleanup, err = b.registerClusterUUIDFetching()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if err := metricreport.SetupMetrics(logp.NewLogger("metrics"), b.Info.Beat, b.Info.Version); err != nil {
		return nil, err
	}

	if !b.Config.Output.IsSet() || !b.Config.Output.Config().Enabled() {
		if !b.Manager.Enabled() {
			return nil, errors.New("no outputs are defined, please define one under the output section")
		}
		logp.Info("output is configured through central management")
	}

	monitors := pipeline.Monitors{
		Metrics:   libbeatMetricsRegistry,
		Telemetry: monitoring.GetNamespace("state").GetRegistry(),
		Logger:    logp.L().Named("publisher"),
		Tracer:    b.Instrumentation.Tracer(),
	}
	outputFactory := b.makeOutputFactory(b.Config.Output)
	publisher, err := pipeline.Load(b.Info, monitors, pipeline.Config{}, nopProcessingSupporter{}, outputFactory)
	if err != nil {
		return nil, fmt.Errorf("error initializing publisher: %w", err)
	}
	b.Publisher = publisher

	// TODO(axw) pass registry into BeatParams, for testing purposes.
	reload.Register.MustRegister("output", b.makeOutputReloader(publisher.OutputReloader()))

	return beatCreator(&b.Beat, b.Config.APMServer)
}

func (b *Beat) Run(ctx context.Context) error {
	defer logp.Sync()
	defer func() {
		if r := recover(); r != nil {
			logp.NewLogger("").Fatalw("exiting due to panic",
				"panic", r,
				zap.Stack("stack"),
			)
		}
	}()
	defer logp.Info("%s stopped.", b.Info.Beat)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Windows: Mark service as stopped.
	// After this is run, a Beat service is considered by the OS to be stopped
	// and another instance of the process can be started.
	// This must be the first deferred cleanup task (last to execute).
	defer svc.NotifyTermination()

	// Try to acquire exclusive lock on data path to prevent another beat instance
	// sharing same data path.
	locker := newLocker(b)
	if err := locker.lock(); err != nil {
		return err
	}
	defer locker.unlock()

	svc.BeforeRun()
	defer svc.Cleanup()

	b.registerMetrics()

	// Start the libbeat API server for serving stats, state, etc.
	var apiServer *api.Server
	if b.Config.HTTP.Enabled() {
		var err error
		apiServer, err = api.NewWithDefaultRoutes(logp.NewLogger(""), b.Config.HTTP, monitoring.GetNamespace)
		if err != nil {
			return fmt.Errorf("could not start the HTTP server for the API: %w", err)
		}
		apiServer.Start()
		defer apiServer.Stop()
		if b.Config.HTTPPprof.Enabled() {
			apiServer.AttachPprof()
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

	logger := logp.NewLogger("")
	done := make(chan struct{})
	go func() {
		defer close(done)
		adjustMaxProcs(ctx, 30*time.Second, diffInfof(logger), logger.Errorf)
	}()
	defer func() { <-done }()

	beater, err := b.createBeater(b.create)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		b.Instrumentation.Tracer().Close()
		beater.Stop()
	}()
	svc.HandleSignals(cancel, cancel)

	logp.Info("%s started.", b.Info.Beat)
	b.Manager.SetStopCallback(cancel)
	return beater.Run(&b.Beat)
}

// registerMetrics registers metrics with the internal monitoring API. This data
// is then exposed through the HTTP monitoring endpoint (e.g. /info and /state)
// and/or pushed to Elasticsearch through the x-pack monitoring feature.
func (b *Beat) registerMetrics() {
	// info
	infoRegistry := monitoring.GetNamespace("info").GetRegistry()
	monitoring.NewString(infoRegistry, "version").Set(b.Info.Version)
	monitoring.NewString(infoRegistry, "beat").Set(b.Info.Beat)
	monitoring.NewString(infoRegistry, "name").Set(b.Info.Name)
	monitoring.NewString(infoRegistry, "hostname").Set(b.Info.Hostname)
	monitoring.NewString(infoRegistry, "uuid").Set(b.Info.ID.String())
	monitoring.NewString(infoRegistry, "ephemeral_id").Set(b.Info.EphemeralID.String())
	monitoring.NewString(infoRegistry, "binary_arch").Set(runtime.GOARCH)
	monitoring.NewString(infoRegistry, "build_commit").Set(vcsRevision)
	monitoring.NewTimestamp(infoRegistry, "build_time").Set(vcsTime)
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

	stateRegistry := monitoring.GetNamespace("state").GetRegistry()

	// state.service
	serviceRegistry := stateRegistry.NewRegistry("service")
	monitoring.NewString(serviceRegistry, "version").Set(b.Info.Version)
	monitoring.NewString(serviceRegistry, "name").Set(b.Info.Beat)
	monitoring.NewString(serviceRegistry, "id").Set(b.Info.ID.String())

	// state.beat
	beatRegistry := stateRegistry.NewRegistry("beat")
	monitoring.NewString(beatRegistry, "name").Set(b.Info.Name)

	// state.host
	monitoring.NewFunc(stateRegistry, "host", host.ReportInfo, monitoring.Report)

	// state.management
	managementRegistry := stateRegistry.NewRegistry("management")
	monitoring.NewBool(managementRegistry, "enabled").Set(b.Manager.Enabled())
}

// registerElasticsearchVerfication registers a global callback to make sure
// the Elasticsearch instance we are connecting to has a valid license, and is
// at least on the same version as APM Server.
//
// registerElasticsearchVerification returns a cleanup function which must be
// called on shutdown.
func (b *Beat) registerElasticsearchVersionCheck() (func(), error) {
	uuid, err := elasticsearch.RegisterGlobalCallback(func(conn *eslegclient.Connection) error {
		if err := licenser.FetchAndVerify(conn); err != nil {
			return err
		}
		esVersion := conn.GetVersion()
		beatVersion, err := libversion.New(b.Info.Version)
		if err != nil {
			return err
		}
		if esVersion.LessThanMajorMinor(beatVersion) {
			return fmt.Errorf(
				"%w Elasticsearch: %s, APM Server: %s",
				elasticsearch.ErrTooOld, esVersion.String(), b.Info.Version,
			)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return func() { elasticsearch.DeregisterGlobalCallback(uuid) }, nil
}

func (b *Beat) makeOutputReloader(outReloader pipeline.OutputReloader) reload.Reloadable {
	return reload.ReloadableFunc(func(config *reload.ConfigWithMeta) error {
		if b.OutputConfigReloader != nil {
			if err := b.OutputConfigReloader.Reload(config); err != nil {
				return err
			}
		}
		return outReloader.Reload(config, b.createOutput)
	})
}

func (b *Beat) makeOutputFactory(
	cfg config.Namespace,
) func(outputs.Observer) (string, outputs.Group, error) {
	return func(outStats outputs.Observer) (string, outputs.Group, error) {
		out, err := b.createOutput(outStats, cfg)
		return cfg.Name(), out, err
	}
}

func (b *Beat) createOutput(stats outputs.Observer, cfg config.Namespace) (outputs.Group, error) {
	if !cfg.IsSet() {
		return outputs.Group{}, nil
	}
	indexSupporter := newSupporter(nil, b.Info, b.rawConfig)
	return outputs.Load(indexSupporter, b.Info, stats, cfg.Name(), cfg.Config())
}

func (b *Beat) registerClusterUUIDFetching() (func(), error) {
	callback := b.clusterUUIDFetchingCallback()
	uuid, err := elasticsearch.RegisterConnectCallback(callback)
	if err != nil {
		return nil, err
	}
	return func() { elasticsearch.DeregisterConnectCallback(uuid) }, nil
}

// Build and return a callback to fetch the Elasticsearch cluster_uuid for monitoring
func (b *Beat) clusterUUIDFetchingCallback() elasticsearch.ConnectCallback {
	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	elasticsearchRegistry := stateRegistry.NewRegistry("outputs.elasticsearch")
	clusterUUIDRegVar := monitoring.NewString(elasticsearchRegistry, "cluster_uuid")

	callback := func(esClient *eslegclient.Connection) error {
		var response struct {
			ClusterUUID string `json:"cluster_uuid"`
		}

		status, body, err := esClient.Request("GET", "/", "", nil, nil)
		if err != nil {
			return fmt.Errorf("error querying /: %w", err)
		}
		if status > 299 {
			return fmt.Errorf("error querying /. Status: %d. Response body: %s", status, body)
		}
		err = json.Unmarshal(body, &response)
		if err != nil {
			return fmt.Errorf("error unmarshaling json when querying /. Body: %s", body)
		}

		clusterUUIDRegVar.Set(response.ClusterUUID)
		return nil
	}

	return callback
}

func (b *Beat) setupMonitoring() (report.Reporter, error) {
	monitoringCfg := b.Config.Monitoring

	monitoringClusterUUID, err := monitoring.GetClusterUUID(monitoringCfg)
	if err != nil {
		return nil, err
	}

	// Expose monitoring.cluster_uuid in state API
	if monitoringClusterUUID != "" {
		stateRegistry := monitoring.GetNamespace("state").GetRegistry()
		monitoringRegistry := stateRegistry.NewRegistry("monitoring")
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
		reporter, err := report.New(b.Info, settings, monitoringCfg, b.Config.Output)
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
	defer logp.Recover("An unexpected error occurred while collecting " +
		"information about the system.")
	log := logp.NewLogger("beat").With(logp.Namespace("system_info"))

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
		"commit":  vcsRevision,
		"time":    vcsTime,
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

type nopProcessingSupporter struct{}

func (nopProcessingSupporter) Close() error {
	return nil
}

func (nopProcessingSupporter) Create(cfg beat.ProcessingConfig, _ bool) (beat.Processor, error) {
	return cfg.Processor, nil
}
