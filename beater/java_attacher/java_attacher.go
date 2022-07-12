package javaattacher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var encounteredJvmCache = make(map[string]*JvmDetails)

// javaAttacher is bundled by the server
var javaAttacher = filepath.FromSlash("./java-attacher.jar")

type JvmDetails struct {
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
	discoveryRules       []DiscoveryRule
	rawDiscoveryRules    []map[string]string
	agentConfigs         map[string]string
	downloadAgentVersion string
}

func (j *JavaAttacher) addDiscoveryRule(rule DiscoveryRule) {
	j.discoveryRules = append(j.discoveryRules, rule)
	j.logger.Debugf("added discovery rule: %v", rule)
}

func New(cfg config.JavaAttacherConfig) JavaAttacher {
	logger := logp.NewLogger("java-attacher")
	attacher := JavaAttacher{
		logger:               logger,
		enabled:              cfg.Enabled,
		agentConfigs:         cfg.Config,
		downloadAgentVersion: cfg.DownloadAgentVersion,
		rawDiscoveryRules:    cfg.DiscoveryRules,
	}
	for _, flag := range cfg.DiscoveryRules {
		for name, value := range flag {
			switch name {
			case "include-all":
				attacher.addDiscoveryRule(IncludeAllRule{})
			case "include-user":
				attacher.addDiscoveryRule(UserDiscoveryRule{user: value, isIncludeRule: true})
			case "exclude-user":
				attacher.addDiscoveryRule(UserDiscoveryRule{user: value, isIncludeRule: false})
			case "include-main":
				attacher.addCmdLineDiscoveryRule(value, true, "include-main")
			case "exclude-main":
				attacher.addCmdLineDiscoveryRule(value, false, "exclude-main")
			case "include-vmarg":
				attacher.addCmdLineDiscoveryRule(value, true, "include-vmarg")
			case "exclude-vmarg":
				attacher.addCmdLineDiscoveryRule(value, false, "exclude-main")
			}
		}
	}
	return attacher
}

func (j *JavaAttacher) addCmdLineDiscoveryRule(regexS string, isIncludeRule bool, argumentName string) {
	regex, err := regexp.Compile(regexS)
	if err != nil {
		j.logger.Errorf("invalid regex for the `%v` argument: %v", argumentName, err)
	}
	if regex != nil {
		j.addDiscoveryRule(CmdLineDiscoveryRule{regex: regex, isIncludeRule: isIncludeRule})
	}
}

func (j *JavaAttacher) findFirstMatch(jvm *JvmDetails) DiscoveryRule {
	for _, rule := range j.discoveryRules {
		if rule.match(jvm) {
			return rule
		}
	}
	return nil
}

func (j *JavaAttacher) Run(ctx context.Context) {
	if !j.enabled {
		j.logger.Debugf("Java Agent attacher is disabled")
		return
	}

	c := make(chan struct{})
	defer close(c)
	go func() {
		for {
			jvms, err := j.discoverJvmsForAttachment(ctx)
			if err != nil {
				j.logger.Infof("error during JVMs discovery: %v", err)
			}
			j.executeForEachJvm(ctx, jvms, j.attach, true, 10*time.Second)
			select {
			case <-ctx.Done():
				return
			case <-c:
				return
			default:
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (j *JavaAttacher) discoverJvmsForAttachment(ctx context.Context) (map[string]*JvmDetails, error) {
	jvms, err := j.discoverAllRunningJavaProcesses(ctx)
	if err != nil {
		if jvms == nil {
			return nil, err
		} else {
			j.logger.Errorf("error during JVMs discovery: %v. Continuing with properly discovered %v JVMs", err, len(jvms))
		}
	}

	// remove stale processes from the cache
	for pid := range encounteredJvmCache {
		_, found := jvms[pid]
		if !found {
			delete(encounteredJvmCache, pid)
		}
	}

	// trying to improve start time accuracy - worth to consider optimizing as this runs every time the loop runs (1 second or so)
	if j.executeForEachJvm(ctx, jvms, j.obtainAccurateStartTime, false, time.Second) {
		j.logger.Errorf("timeout trying to find accurate start time for %v jvms", len(jvms))
	}

	j.filterCached(jvms)

	if j.executeForEachJvm(ctx, jvms, j.obtainCommandLineArgs, false, time.Second) {
		j.logger.Errorf("timeout trying to find command line args for %v jvms", len(jvms))
	}

	// todo: decide whether we have enough advantage to do that within the integration instead of doing it within the attacher
	j.filterByDiscoveryRules(jvms)

	if j.executeForEachJvm(ctx, jvms, j.verifyJvmExecutable, true, time.Second) {
		j.logger.Errorf("timeout trying to verify Java executables for %v jvms", len(jvms))
	}

	for _, jvm := range jvms {
		fmt.Printf("Found a Java process (PID %v) of version %v that was started at %v by '%v' using command '%v'\n",
			jvm.pid, jvm.version, jvm.startTime, jvm.user, jvm.command)
	}
	return jvms, nil
}

func (j *JavaAttacher) discoverAllRunningJavaProcesses(ctx context.Context) (map[string]*JvmDetails, error) {
	jvms := make(map[string]*JvmDetails)

	// We use `start`, which is very inaccurate only as a fallback, expecting it to be replaced with an accurate start time later on.
	// An example for the `start` time format: "4:01PM". We may decide that with the combination of PID, it is safe enough for
	// uniqueness determination for caching purposes, especially if we remove stale PIDs when we don't see them.
	// We avoid using `lstart` here because it is not one of the standard ps columns, and because it may have different format in different
	// systems. In addition, it would complicate output parsing as its format is full of spaces, which we rely on as a delimiter between
	// output line parts.
	cmd := exec.CommandContext(ctx, "ps", "-A", "-ww", "-o", "user,uid,gid,pid,start,comm")
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
		if len(psOutputLineParts) > 5 {
			command := psOutputLineParts[5]
			if strings.Contains(strings.ToLower(command), "java") {
				pid := psOutputLineParts[3]
				jvms[pid] = &JvmDetails{
					psOutputLineParts[0],
					psOutputLineParts[1],
					psOutputLineParts[2],
					pid,
					psOutputLineParts[4],
					psOutputLineParts[5],
					"unknown",
					"",
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		j.logger.Errorf("error reading output for command '%v': %v", strings.Join(cmd.Args, " "), err)
	}
	err = cmd.Wait()
	return jvms, err
}

func (j *JavaAttacher) executeForEachJvm(ctx context.Context, jvms map[string]*JvmDetails,
	executable func(ctx context.Context, jvm *JvmDetails) error, removeOnError bool, timeout time.Duration) bool {
	var wg sync.WaitGroup
	for pid, jvm := range jvms {
		wg.Add(1)
		go func(jvm *JvmDetails, pid string) {
			err := executable(ctx, jvm)
			if err != nil {
				j.logger.Debug(err)
				if removeOnError {
					delete(jvms, pid)
				}
			}
			wg.Done()
		}(jvm, pid)
	}
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-ctx.Done():
		return false
	case <-c:
		return false
	case <-time.After(timeout):
		return true // timed out
	}
}

func (j *JavaAttacher) obtainAccurateStartTime(ctx context.Context, jvmDetails *JvmDetails) error {
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
		j.logger.Errorf("error obtaining accurate start time for process %v using 'ps -o lstart=': %v", err)
	}
	return cmd.Wait()
}

func (j *JavaAttacher) obtainCommandLineArgs(ctx context.Context, jvmDetails *JvmDetails) error {
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
	}
	if err := scanner.Err(); err != nil {
		j.logger.Errorf("error obtaining command line args for process %v using 'ps -o command=': %v", err)
	}
	return cmd.Wait()
}

func (j *JavaAttacher) filterCached(jvms map[string]*JvmDetails) {
	for pid, jvm := range jvms {
		cachedJvmDetails, found := encounteredJvmCache[pid]
		if !found || cachedJvmDetails.startTime != jvm.startTime {
			// this is a JVM not yet encountered - add to cache
			encounteredJvmCache[pid] = jvm
		} else {
			delete(jvms, pid)
		}
	}
}

func (j *JavaAttacher) filterByDiscoveryRules(jvms map[string]*JvmDetails) {
	for pid, jvm := range jvms {
		matchRule := j.findFirstMatch(jvm)
		if matchRule != nil {
			if matchRule.include() {
				j.logger.Debugf("include rule %v matches for JVM %v", matchRule, jvm)
			} else {
				delete(jvms, pid)
				j.logger.Debugf("exclude rule %v matches for JVM %v", matchRule, jvm)
			}
		} else {
			delete(jvms, pid)
			j.logger.Debugf("no rule matches JVM %v", jvm)
		}
	}
}

func (j *JavaAttacher) verifyJvmExecutable(ctx context.Context, jvm *JvmDetails) error {
	// `java -version` prints to the error stream, so we want to insist on it, we need to use StderrPipe instead of StdoutPipe
	cmd := exec.CommandContext(ctx, jvm.command, "--version")
	err := j.setRunAsUser(jvm, cmd)
	if err != nil {
		j.logger.Errorf("failed to run `java --version` as user %v: %v. Trying to execute as current user,", jvm.user, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		// only use first line of the output
		jvm.version = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		j.logger.Errorf("error reading output of command '%v' when running as user %v: %v", strings.Join(cmd.Args, " "), jvm.user, err)
		fmt.Println(err)
	}
	return cmd.Wait()
}

func (j *JavaAttacher) setRunAsUser(jvm *JvmDetails, cmd *exec.Cmd) error {
	currentUser, err := user.Current()
	if err != nil {
		j.logger.Infof("failed to get the current user: %v", err)
	} else {
		j.logger.Debugf("current user: %v", currentUser)
	}
	if currentUser.Gid != jvm.gid || currentUser.Uid != jvm.uid {
		uid, err := strconv.ParseInt(jvm.uid, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid UID '%v': %v", jvm.uid, err)
		}
		gid, err := strconv.ParseInt(jvm.gid, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid GID '%v': %v", jvm.gid, err)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
	return nil
}

func (j *JavaAttacher) attach(ctx context.Context, jvm *JvmDetails) error {
	cmd := j.build(ctx, jvm)
	err := j.setRunAsUser(jvm, cmd)
	if err != nil {
		j.logger.Errorf("failed to attach to JVM pid %v as user %v: %v. Trying to attach as current user,", jvm.pid, jvm.user, err)
	}
	j.logger.Infof("starting java attacher with command: %s", strings.Join(cmd.Args, " "))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot read from attacher standard output: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot read from attacher standard error: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start attacher standard error: %v", err)
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
				j.logger.Errorf("error unmarshaling attacher logs: %v", err)
				continue
			}
			switch b.LogLevel {
			case "FATAL", "ERROR":
				j.logger.Error(b.Message)
			case "WARN":
				j.logger.Warn(b.Message)
			case "INFO":
				j.logger.Info(b.Message)
			case "DEBUG", "TRACE":
				fmt.Println(b.Message)
				j.logger.Debug(b.Message)
			default:
				j.logger.Errorf("unrecognized java-attacher log.level: %s", b.LogLevel)
			}
		}
		if err := scanner.Err(); err != nil {
			j.logger.Errorf("error scanning attacher logs: %v", err)
		}
	}()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
		j.logger.Errorf("error running attacher: %v", scanner.Text())
	}

	return cmd.Wait()
}

func (j JavaAttacher) build(ctx context.Context, jvm *JvmDetails) *exec.Cmd {
	args := append([]string{"-jar", javaAttacher}, j.formatArgs(jvm)...)
	return exec.CommandContext(ctx, jvm.command, args...)
}

func (j JavaAttacher) formatArgs(jvm *JvmDetails) []string {
	args := []string{"--log-level", "debug"}
	args = append(args, "--include-pid", jvm.pid)

	if j.downloadAgentVersion != "" {
		args = append(args, "--download-agent-version", j.downloadAgentVersion)
	}

	// todo: decide if we apply rules in the integration level or the attacher level
	//for _, flag := range j.rawDiscoveryRules {
	//	for name, value := range flag {
	//		args = append(args, "--"+name, value)
	//	}
	//}

	cfg := make([]string, 0, len(j.agentConfigs))
	for k, v := range j.agentConfigs {
		cfg = append(cfg, "--config", k+"="+v)
	}

	return append(args, cfg...)
}
