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
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/beater/config"
)

// javaAttacher is bundled by the server
var javaAttacher = filepath.FromSlash("./java-attacher.jar")

type jvmDetails struct {
	user        string
	uid         string
	gid         string
	pid         string
	startTime   string
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
	javaBin              string
	jvmCache             map[string]*jvmDetails
}

func New(cfg config.JavaAttacherConfig) (*JavaAttacher, error) {
	logger := logp.NewLogger("java-attacher")
	if _, err := os.Stat(javaAttacher); err != nil {
		return nil, err
	}
	attacher := &JavaAttacher{
		logger:               logger,
		enabled:              cfg.Enabled,
		agentConfigs:         cfg.Config,
		downloadAgentVersion: cfg.DownloadAgentVersion,
		rawDiscoveryRules:    cfg.DiscoveryRules,
		javaBin:              cfg.JavaBin,
		jvmCache:             make(map[string]*jvmDetails),
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

	if runtime.GOOS == "windows" {
		// On Windows, we run the attacher by a JVM available to the
		// current user in continuous mode.
		if err := j.discoverJavaExecutable(); err != nil {
			return err
		}
		return j.runAttacherContinuous(ctx)
	}

	// Non-Windows: run discovery and attachment until context is closed.
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
			// Error is non-fatal; try again next time.
			j.logger.Errorf("error during JVMs discovery: %v", err)
			continue
		}
		if err := j.foreachJVM(ctx, jvms, j.attachJVM, true, 30*time.Second); err != nil {
			// Error is non-fatal; try again next time.
			j.logger.Errorf("JVM attachment failed: %s", err)
		}
	}
}

func (j *JavaAttacher) discoverJavaExecutable() error {
	if j.javaBin == "" {
		if jh := os.Getenv("JAVA_HOME"); jh != "" {
			j.javaBin = filepath.Join(jh, "bin", "java")
		} else {
			bin, err := exec.LookPath("java")
			if err != nil {
				return fmt.Errorf("no java binary found: %w", err)
			}
			j.javaBin = bin
		}
	} else {
		// Ensure we're using the correct separators for the system
		// running apm-server
		j.javaBin = filepath.FromSlash(j.javaBin)
	}
	return nil
}

// this function blocks until discovery has ended, an error occurred or the context had been cancelled
func (j *JavaAttacher) discoverJVMsForAttachment(ctx context.Context) (map[string]*jvmDetails, error) {
	// TODO: use github.com/elastic/go-sysinfo instead of shelling out to `ps`.

	jvms, err := j.discoverAllRunningJavaProcesses(ctx)
	if err != nil {
		return nil, err
	}

	// remove stale processes from the cache
	for pid := range j.jvmCache {
		if _, found := jvms[pid]; !found {
			delete(j.jvmCache, pid)
		}
	}

	// trying to improve start time accuracy - worth to consider optimizing as this runs every time the loop runs (1 second or so)
	if err := j.foreachJVM(ctx, jvms, j.obtainAccurateStartTime, false, time.Second); err != nil {
		j.logger.Infof("finding accurate start time for %v jvms did not finish successfully: %v", len(jvms), err)
	}

	// Remove any JVMs that have previously been discovered.
	j.filterCached(jvms)

	if err := j.foreachJVM(ctx, jvms, j.obtainCommandLineArgs, false, time.Second); err != nil {
		j.logger.Infof("finding command line args for %v jvms did not finish successfully: %v", len(jvms), err)
	}

	// todo: decide whether we have enough advantage to do that within the integration instead of doing it within the attacher
	j.filterByDiscoveryRules(jvms)

	if err := j.foreachJVM(ctx, jvms, j.verifyJVMExecutable, true, time.Second); err != nil {
		j.logger.Infof("verifying Java executables for %v jvms did not finish successfully: %v", len(jvms), err)
	}
	if len(jvms) > 0 {
		j.printJVMs(jvms, "Java executable verification")
	}

	return jvms, nil
}

func (j *JavaAttacher) printJVMs(jvms map[string]*jvmDetails, stepName string) {
	j.logger.Debugf("%v processes are candidates for Java agent attachment after %v:", len(jvms), stepName)
	for _, jvm := range jvms {
		j.logger.Debugf("PID: %v, version: %v, start-time: %v, user: '%v', command: '%v'",
			jvm.pid, jvm.version, jvm.startTime, jvm.user, jvm.command)
	}
}

// this function blocks until discovery has ended, an error occurred or the context had been cancelled
func (j *JavaAttacher) discoverAllRunningJavaProcesses(ctx context.Context) (map[string]*jvmDetails, error) {
	jvms := make(map[string]*jvmDetails)

	// We can't discover start time at this point because both `start` and `lstart` don't follow a strict-enough format,
	// which may interfere with output parsing.
	cmd := exec.CommandContext(ctx, "ps", "-A", "-ww", "-o", "user=,uid=,gid=,pid=,comm=")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		psOutputLineParts := strings.Fields(line)
		// we cannot rely on exact number of parts because the command output may contain spaces, however, since it is printed last,
		// we are ok with just taking the command's first part in the case of Java processes
		if len(psOutputLineParts) > 4 {
			command := psOutputLineParts[4]
			if strings.Contains(strings.ToLower(command), "java") {
				pid := psOutputLineParts[3]
				jvms[pid] = &jvmDetails{
					psOutputLineParts[0],
					psOutputLineParts[1],
					psOutputLineParts[2],
					pid,
					"unknown",
					command,
					"unknown",
					"",
				}
			}
		} else {
			j.logger.Errorf("Unexpected number of output fields (%v) from command '%v'", len(psOutputLineParts), strings.Join(cmd.Args, " "))
		}
	}
	if err := scanner.Err(); err != nil {
		j.logger.Errorf("error reading output for command %q: %v", strings.Join(cmd.Args, " "), err)
	}
	err = cmd.Wait()
	if err != nil {
		j.logger.Errorf("error executing command %q: %v", strings.Join(cmd.Args, " "), err)
	}
	return jvms, nil
}

// foreachJVM calls f for each JVM in jvms concurrently,
// returning when one of the following is true:
//   1. The function completes for each JVM
//   2. The timeout is reached.
//   3.	The context is cancelled.
//
// If removeOnError is true and f returns an error, that
// JVM will be removed from the map.
func (j *JavaAttacher) foreachJVM(
	ctx context.Context,
	jvms map[string]*jvmDetails,
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

func (j *JavaAttacher) obtainAccurateStartTime(ctx context.Context, jvmDetails *jvmDetails) error {
	// An example output for "ps -p <PID> -o lstart=": "Mon Jul  4 16:01:48 2022"
	// Optional optimization: chain all PIDs and execute a single command, e.g. "ps -p 18246 -p 680 -o lstart="
	cmd := exec.CommandContext(ctx, "ps", "-p", fmt.Sprint(jvmDetails.pid), "-o", "lstart=")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		jvmDetails.startTime = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		j.logger.Errorf("error obtaining accurate start time for process %s using 'ps -o lstart=': %v", jvmDetails.pid, err)
	}
	return cmd.Wait()
}

func (j *JavaAttacher) obtainCommandLineArgs(ctx context.Context, jvmDetails *jvmDetails) error {
	cmd := exec.CommandContext(ctx, "ps", "-p", fmt.Sprint(jvmDetails.pid), "-ww", "-o", "command=")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		jvmDetails.cmdLineArgs = scanner.Text()
		// at least in some Linux distributions, the `comm` option may only contain the executable and not the full path,
		// so we should replace jvmDetails.command with the first part of the output of ps -o command
		jvmDetails.command = strings.Fields(jvmDetails.cmdLineArgs)[0]
	}
	if err := scanner.Err(); err != nil {
		j.logger.Errorf("error obtaining command line args for process %s using 'ps -o command=': %v", jvmDetails.pid, err)
	}
	return cmd.Wait()
}

func (j *JavaAttacher) filterCached(jvms map[string]*jvmDetails) {
	for pid, jvm := range jvms {
		cachedjvmDetails, found := j.jvmCache[pid]
		if !found || cachedjvmDetails.startTime != jvm.startTime {
			// this is a JVM not yet encountered - add to cache
			j.jvmCache[pid] = jvm
		} else {
			delete(jvms, pid)
		}
	}
}

func (j *JavaAttacher) filterByDiscoveryRules(jvms map[string]*jvmDetails) {
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
// once the agent is attached; it is not long-running.
func (j *JavaAttacher) attachJVM(ctx context.Context, jvm *jvmDetails) error {
	cmd := j.attachJVMCommand(ctx, jvm)
	return runAttacherCommand(ctx, cmd, j.logger)
}

func (j *JavaAttacher) attachJVMCommand(ctx context.Context, jvm *jvmDetails) *exec.Cmd {
	return j.attacherCommand(ctx, jvm.command, "--include-pid", jvm.pid)
}

// attachJVM runs the Java agent attacher in continuous mode, delegating
// JVM discovery to the attacher.
func (j *JavaAttacher) runAttacherContinuous(ctx context.Context) error {
	cmd := j.runAttacherContinuousCommand(ctx)
	return runAttacherCommand(ctx, cmd, j.logger)
}

func (j *JavaAttacher) runAttacherContinuousCommand(ctx context.Context) *exec.Cmd {
	args := []string{"--continuous"}
	for _, flag := range j.rawDiscoveryRules {
		for name, value := range flag {
			args = append(args, "--"+name, value)
		}
	}
	return j.attacherCommand(ctx, j.javaBin, args...)
}

// attacherCommand returns an os/exec.Cmd that will run the Java agent attacher
// with the given java command. If java is empty, then j.javaBin will be used.
// Any supplied arguments will be appended to the command line.
func (j *JavaAttacher) attacherCommand(ctx context.Context, java string, extraArgs ...string) *exec.Cmd {
	args := []string{
		"-jar", javaAttacher,
		"--log-level", "debug",
	}
	if j.downloadAgentVersion != "" {
		args = append(args, "--download-agent-version", j.downloadAgentVersion)
	}
	for k, v := range j.agentConfigs {
		args = append(args, "--config", k+"="+v)
	}
	args = append(args, extraArgs...)
	if java == "" {
		java = j.javaBin
	}
	return exec.CommandContext(ctx, java, args...)
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
