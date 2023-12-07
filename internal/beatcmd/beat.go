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
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/api"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/licenser"
	"github.com/elastic/beats/v7/libbeat/management"
	"github.com/elastic/beats/v7/libbeat/monitoring/report"
	"github.com/elastic/beats/v7/libbeat/monitoring/report/log"
	"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/pprof"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/file"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/monitoring/report/buffer"
	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/elastic/elastic-agent-libs/service"
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

// Beat provides the runnable and configurable instance of a beat.
type Beat struct {
	beat.Beat

	Config *Config

	rawConfig *config.C
	newRunner NewRunnerFunc
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
	b := &Beat{
		Beat: beat.Beat{
			Info: beat.Info{
				Beat:            "apm-server",
				ElasticLicensed: args.ElasticLicensed,
				IndexPrefix:     "apm-server",
				Version:         version.Version,
				Name:            beatName,
				Hostname:        hostname,
				StartTime:       time.Now(),
				EphemeralID:     metricreport.EphemeralID(),
			},
			Keystore:   keystore,
			Config:     &beat.BeatConfig{Output: cfg.Output},
			BeatConfig: cfg.APMServer,
		},
		Config:    cfg,
		newRunner: args.NewRunner,
		rawConfig: rawConfig,
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

	// log paths values to help with troubleshooting
	logp.Info(paths.Paths.String())

	// Load the unique ID and "first start" info from meta.json.
	metaPath := paths.Resolve(paths.Data, "meta.json")
	if err := b.loadMeta(metaPath); err != nil {
		return err
	}
	logp.Info("Beat ID: %v", b.Info.ID)

	// Initialize central config manager.
	manager, err := management.NewManager(b.Config.Management, reload.RegisterV2)
	if err != nil {
		return err
	}
	b.Manager = manager

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

	logger := logp.NewLogger("")

	if runtime.GOOS == "darwin" {
		if host, err := sysinfo.Host(); err != nil {
			logger.Warnf("failed to retrieve kernel version, ignoring potential deprecation warning: %v", err)
		} else if strings.HasPrefix(host.Info().KernelVersion, "19.") {
			// macOS 10.15.x (catalina) means darwin kernel 19.y
			logger.Warn("deprecation notice: support for macOS 10.15 will be removed in an upcoming version")
		}
	}

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
		apiServer, err = api.NewWithDefaultRoutes(logp.NewLogger(""), b.Config.HTTP, monitoring.GetNamespace)
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

	g.Go(func() error {
		return adjustMaxProcs(ctx, 30*time.Second, logger)
	})

	logSystemInfo(b.Info)

	cleanup, err := b.registerElasticsearchVersionCheck()
	if err != nil {
		return err
	}
	defer cleanup()

	cleanup, err = b.registerClusterUUIDFetching()
	if err != nil {
		return err
	}
	defer cleanup()

	if err := metricreport.SetupMetrics(logp.NewLogger("metrics"), b.Info.Beat, b.Info.Version); err != nil {
		return err
	}

	if b.Manager.Enabled() {
		reloader, err := NewReloader(b.Info, b.newRunner)
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
			Config: b.rawConfig,
			Info:   b.Info,
			Logger: logp.NewLogger(""),
		})
		if err != nil {
			return err
		}
		g.Go(func() error { return runner.Run(ctx) })
	}
	logp.Info("%s started.", b.Info.Beat)
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
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
	monitoring.NewFunc(stateRegistry, "host", host.ReportInfo("" /* don't use FQDN */), monitoring.Report)

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
