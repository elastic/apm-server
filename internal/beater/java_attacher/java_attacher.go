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

package javaattacher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-sysinfo"

	"github.com/elastic/apm-server/internal/beater/config"
)

const javawExe = "javaw.exe"
const javaExe = "java.exe"

const bundledJavaAttacher = "java-attacher.jar"

type jvmDetails struct {
	user        string
	uid         string
	gid         string
	pid         int
	startTime   time.Time
	command     string
	version     string
	cmdLineArgs string
}

type JavaAttacher struct {
	logger               *logp.Logger
	enabled              bool
	discoveryRules       []discoveryRule
	rawDiscoveryRules    []map[string]string
	agentConfigs         map[string]string
	downloadAgentVersion string
	jvmCache             map[int]*jvmDetails
	uidToAttacherJar     map[string]string
	tmpDirs              []string
	tmpAttacherLock      sync.Mutex
}

func New(cfg config.JavaAttacherConfig) (*JavaAttacher, error) {
	logger := logp.NewLogger("java-attacher")
	if _, err := os.Stat(bundledJavaAttacher); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("APM Java agent attacher needs to be explicitly enabled")
	}
	if len(cfg.DiscoveryRules) == 0 {
		return nil, fmt.Errorf("APM Java agent attacher is enabled but no discovery rules are configured, " +
			"meaning no JVM can be attached to")
	}
	attacher := &JavaAttacher{
		logger:               logger,
		enabled:              cfg.Enabled,
		agentConfigs:         cfg.Config,
		downloadAgentVersion: cfg.DownloadAgentVersion,
		rawDiscoveryRules:    cfg.DiscoveryRules,
		jvmCache:             make(map[int]*jvmDetails),
		uidToAttacherJar:     make(map[string]string),
	}
	for _, flag := range cfg.DiscoveryRules {
		for name, value := range flag {
			switch name {
			case "include-all":
				attacher.addDiscoveryRule(includeAllRule{})
			case "include-user":
				attacher.addUserDiscoveryRule(value, true)
			case "exclude-user":
				attacher.addUserDiscoveryRule(value, false)
			case "include-main":
				attacher.addCmdLineDiscoveryRule(value, true, "include-main")
			case "exclude-main":
				attacher.addCmdLineDiscoveryRule(value, false, "exclude-main")
			case "include-vmarg":
				attacher.addCmdLineDiscoveryRule(value, true, "include-vmarg")
			case "exclude-vmarg":
				attacher.addCmdLineDiscoveryRule(value, false, "exclude-main")
			default:
				logger.Warnf("Ignoring unknown discovery rule %q", name)
			}
		}
	}
	return attacher, nil
}

func (j *JavaAttacher) addUserDiscoveryRule(user string, isIncludeRule bool) {
	j.addDiscoveryRule(&userDiscoveryRule{user: user, isIncludeRule: isIncludeRule})
}

func (j *JavaAttacher) addCmdLineDiscoveryRule(regexS string, isIncludeRule bool, argumentName string) {
	regex, err := regexp.Compile(regexS)
	if err != nil {
		j.logger.Errorf("invalid regex for the %q argument: %v", argumentName, err)
		return
	}
	j.addDiscoveryRule(&cmdLineDiscoveryRule{
		regex:         regex,
		isIncludeRule: isIncludeRule,
		argumentName:  argumentName,
	})
}

func (j *JavaAttacher) addDiscoveryRule(rule discoveryRule) {
	j.discoveryRules = append(j.discoveryRules, rule)
	j.logger.Debugf("added discovery rule: %s", rule)
}

func (j *JavaAttacher) findFirstMatch(jvm *jvmDetails) discoveryRule {
	for _, rule := range j.discoveryRules {
		if rule.match(jvm) {
			return rule
		}
	}
	return nil
}

func (j *JavaAttacher) Run(ctx context.Context) error {
	if !j.enabled {
		return fmt.Errorf("java attacher is disabled")
	}

	defer j.cleanResources()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		jvms, err := j.discoverJVMsForAttachment(ctx)
		if err != nil {
			// Error is non-fatal; try again next time. Errors related to specific JVMs should not reoccur as we cache encountered JVMs.
			j.logger.Errorf("error during JVMs discovery: %v", err)
			continue
		}
		if err := j.foreachJVM(ctx, jvms, j.attachJVM, false, 2*time.Minute); err != nil {
			// Error is non-fatal; try again next time.
			j.logger.Errorf("JVM attachment failed: %s", err)
		}
	}
}

// discoverJVMsForAttachment blocks until discovery ends, an error is received or the context is done.
func (j *JavaAttacher) discoverJVMsForAttachment(ctx context.Context) (map[int]*jvmDetails, error) {
	jvms, err := j.discoverAllRunningJavaProcesses()
	if err != nil {
		return nil, err
	}

	// remove stale processes from the cache
	for pid := range j.jvmCache {
		if _, found := jvms[pid]; !found {
			delete(j.jvmCache, pid)
		}
	}

	// Remove any JVMs that have previously been discovered.
	j.filterCached(jvms)

	j.filterByDiscoveryRules(jvms)

	if err := j.foreachJVM(ctx, jvms, j.verifyJVMExecutable, true, time.Second); err != nil {
		// This is non-fatal: if the Java executable could not be verified for
		// one of the JVM processes, then that process will be removed from the
		// map and excluded from attachment.
		j.logger.Infof("verifying Java executables for JVMs did not finish successfully: %s", err)
	}
	if len(jvms) > 0 {
		j.logger.Debugf("%v processes are candidates for Java agent attachment after Java executable verification:", len(jvms))
		for _, jvm := range jvms {
			j.logger.Debugf(
				"PID: %v, version: %v, start-time: %s, user: %q, command: %q",
				jvm.pid, jvm.version, jvm.startTime, jvm.user, jvm.command,
			)
		}
	}

	return jvms, nil
}

// discoverAllRunningJavaProcesses returns a map of PIDs to running Java processes,
// by looking for processes with "java" in the command.
func (j *JavaAttacher) discoverAllRunningJavaProcesses() (map[int]*jvmDetails, error) {
	processes, err := sysinfo.Processes()
	if err != nil {
		return nil, err
	}
	jvms := make(map[int]*jvmDetails)
	for _, proc := range processes {
		pid := proc.PID()
		info, err := proc.Info()
		if err != nil {
			// We intentionally do not log this error, because it is expected that when
			// apm-server runs as an unprivileged process, it cannot obtain information
			// about every process on the system.
			continue
		}
		if !strings.Contains(strings.ToLower(info.Exe), "java") {
			continue
		}
		userInfo, err := proc.User()
		if err != nil {
			j.logger.Warnf("failed to obtain user for process %d, ignoring", pid)
			continue
		}
		uid := userInfo.EUID
		if uid == "" {
			uid = userInfo.UID
		}
		gid := userInfo.EGID
		if gid == "" {
			gid = userInfo.GID
		}

		var username string
		if jvmUser, err := user.LookupId(uid); err != nil {
			j.logger.Warnf("failed to obtain username for user %s (process %d), using numerical ID", uid, pid)
			username = userInfo.EUID
		} else {
			username = jvmUser.Username
		}

		jvms[pid] = &jvmDetails{
			user:        username,
			uid:         uid,
			gid:         gid,
			pid:         pid,
			startTime:   info.StartTime,
			command:     normalizeJavaCommand(info.Exe),
			version:     "unknown", // filled in later
			cmdLineArgs: strings.Join(info.Args, " "),
		}
	}
	return jvms, nil
}

func normalizeJavaCommand(command string) string {
	if strings.HasSuffix(command, javawExe) {
		command = strings.TrimSuffix(command, javawExe)
		command = command + javaExe
	}
	return command
}

// foreachJVM calls f for each JVM in jvms concurrently,
// returning when one of the following is true:
//  1. The function completes for each JVM
//  2. The timeout is reached.
//  3. The context is cancelled.
//
// If removeOnError is true and f returns an error, that
// JVM will be removed from the map.
func (j *JavaAttacher) foreachJVM(
	ctx context.Context,
	jvms map[int]*jvmDetails,
	f func(ctx context.Context, jvm *jvmDetails) error,
	removeOnError bool,
	timeout time.Duration,
) error {
	if len(jvms) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// mutex for synchronisation removal on error. This being
	// local assumes jvms is not accessed concurrently by any
	// other goroutines.
	var mu sync.Mutex

	var g errgroup.Group
	for pid, jvm := range jvms {
		pid, jvm := pid, jvm // copy for closure
		g.Go(func() error {
			err := f(ctx, jvm)
			if err != nil {
				j.logger.Error(err)
				if removeOnError {
					mu.Lock()
					delete(jvms, pid)
					mu.Unlock()
				}
			}
			return err
		})
	}
	return g.Wait()
}

func (j *JavaAttacher) filterCached(jvms map[int]*jvmDetails) {
	for pid, jvm := range jvms {
		cachedJvmDetails, found := j.jvmCache[pid]
		if !found || !cachedJvmDetails.startTime.Equal(jvm.startTime) {
			// this is a JVM not yet encountered - add to cache
			j.jvmCache[pid] = jvm
		} else {
			delete(jvms, pid)
		}
	}
}

func (j *JavaAttacher) filterByDiscoveryRules(jvms map[int]*jvmDetails) {
	for pid, jvm := range jvms {
		matchRule := j.findFirstMatch(jvm)
		if matchRule == nil {
			delete(jvms, pid)
			j.logger.Debugf("no rule matches JVM %v", jvm)
			continue
		}
		if matchRule.include() {
			j.logger.Debugf("include rule %q matches for JVM %v", matchRule, jvm)
		} else {
			delete(jvms, pid)
			j.logger.Debugf("exclude rule %q matches for JVM %v", matchRule, jvm)
		}
	}
}

func (j *JavaAttacher) verifyJVMExecutable(ctx context.Context, jvm *jvmDetails) error {
	// NOTE: we use --version (double dash), not -version (single dash) to ensure output is sent to stdout.
	cmd := exec.CommandContext(ctx, jvm.command, "--version")
	if err := j.setRunAsUser(jvm, cmd); err != nil {
		j.logger.Warnf("Failed to run `java --version` as user %q: %v. Trying to execute as current user,", jvm.user, err)
	}
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("java --version failed: %w (%s)", err, output)
	}
	firstLine := strings.TrimSpace(strings.SplitAfterN(string(output), "\n", 2)[0])
	jvm.version = firstLine
	return nil
}

// attachJVM runs the Java agent attacher for a specific JVM. This will return
// once the agent is attached or the context is closed; it may be blocking for long and should be called with a timeout context.
func (j *JavaAttacher) attachJVM(ctx context.Context, jvm *jvmDetails) error {
	cmd := j.attachJVMCommand(ctx, jvm)
	if cmd == nil {
		return nil
	}
	if err := j.setRunAsUser(jvm, cmd); err != nil {
		j.logger.Warnf("Failed to attach as user %q: %v. Trying to attach as current user,", jvm.user, err)
	}
	return runAttacherCommand(ctx, cmd, j.logger)
}

// attachJVMCommand constructs an attacher command for the provided jvmDetails.
// NOTE: this method may have side effects, including the creation of a tmp directory with a copy of the attacher jar (non-Windows),
// as well as a corresponding status change to this JavaAttacher, where the created tmp dir and jar paths are cached.
func (j *JavaAttacher) attachJVMCommand(ctx context.Context, jvm *jvmDetails) *exec.Cmd {
	attacherJar := j.getAttacherJar(jvm.uid)
	if attacherJar == "" {
		// If the attacher jar is empty, this means there was an error while
		// attempting to copy the jar to a temporary directory. In this case
		// we should return nil, which will cause attachJVM to skip the process.
		return nil
	}
	args := []string{
		"-jar", attacherJar,
		"--log-level", "debug",
		"--config", "activation_method=FLEET",
		"--include-pid", strconv.Itoa(jvm.pid),
	}
	if j.downloadAgentVersion != "" {
		args = append(args, "--download-agent-version", j.downloadAgentVersion)
	}
	for k, v := range j.agentConfigs {
		args = append(args, "--config", k+"="+v)
	}
	return exec.CommandContext(ctx, jvm.command, args...)
}

func runAttacherCommand(ctx context.Context, cmd *exec.Cmd, logger *logp.Logger) error {
	logger.Infof("starting java attacher with command: %s", strings.Join(cmd.Args, " "))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot read from attacher standard output: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot read from attacher standard error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start attacher standard error: %w", err)
	}

	donec := make(chan struct{})
	defer close(donec)
	go func() {
		scanner := bufio.NewScanner(stdout)
		b := struct {
			LogLevel string `json:"log.level"`
			Message  string `json:"message"`
		}{}
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case <-donec:
				return
			default:
			}
			if err := json.Unmarshal(scanner.Bytes(), &b); err != nil {
				logger.Debugf("error unmarshaling attacher log line (probably not ECS-formatted): %v", err)
				logger.Debugf(scanner.Text())
				continue
			}
			switch b.LogLevel {
			case "FATAL", "ERROR":
				logger.Error(b.Message)
			case "WARN":
				logger.Warn(b.Message)
			case "INFO":
				logger.Info(b.Message)
			case "DEBUG", "TRACE":
				logger.Debug(b.Message)
			default:
				logger.Errorf("unrecognized java-attacher log.level: %s", b.LogLevel)
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Errorf("error reading attacher logs: %v", err)
		}
	}()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		logger.Errorf("error running attacher: %v", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Errorf("error reading attacher error output: %v", err)
	}
	return cmd.Wait()
}
