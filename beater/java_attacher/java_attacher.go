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
	"strings"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/config"
)

type JavaAttacher struct {
	cfg      config.JavaAttacherConfig
	logLevel string
	logger   *logp.Logger
	logfn    logfn
}

type logfn func(...interface{})

func New(cfg config.JavaAttacherConfig, logLevel string) (JavaAttacher, error) {
	if cfg.JavaBin == "" {
		if jh := os.Getenv("JAVA_HOME"); jh != "" {
			cfg.JavaBin = filepath.Join(jh, "/bin/java")
		} else {
			bin, err := exec.LookPath("java")
			if err != nil {
				return JavaAttacher{}, fmt.Errorf("no java binary found: %v", err)
			}
			cfg.JavaBin = bin
		}
	} else {
		// Ensure we're using the correct separators for the system
		// running apm-server
		cfg.JavaBin = filepath.FromSlash(cfg.JavaBin)
	}
	logger := logp.NewLogger("java-attacher")
	return JavaAttacher{
		cfg:      cfg,
		logLevel: logLevel,
		logger:   logger,
		logfn:    setLogfn(logger, logLevel),
	}, nil
}

// javaAttacher is bundled by the server
// TODO: Figure out the real path
const javaAttacher = "/bin/apm-agent-attach-cli-1.24.0-slim.jar"

func (j JavaAttacher) Run(ctx context.Context) error {
	cmd := j.build(ctx)
	j.logger.Debugf("starting java attacher with command: %s", strings.Join(cmd.Args, " "))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	donec := make(chan struct{})
	defer close(donec)
	go func() {
		scanner := bufio.NewScanner(stdout)
		b := struct {
			Message string `json:"message"`
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
			j.logfn(b.Message)
		}
		if err := scanner.Err(); err != nil {
			j.logger.Errorf("error scanning attacher logs: %v", err)
		}
	}()

	return cmd.Wait()
}

func (j JavaAttacher) build(ctx context.Context) *exec.Cmd {
	args := append([]string{"-jar", javaAttacher}, j.formatArgs()...)
	return exec.CommandContext(ctx, j.cfg.JavaBin, args...)
}

func (j JavaAttacher) formatArgs() []string {
	args := []string{"--continuous", "--log-level", j.logLevel}

	for _, flag := range j.cfg.DiscoveryRules {
		for name, value := range flag {
			args = append(args, makeArg("--"+name, value))
		}
	}
	for k, v := range j.cfg.Config {
		args = append(args, "--config "+k+"="+v)
	}

	return args
}

func makeArg(flagName string, args ...string) string {
	return flagName + " " + strings.Join(args, " ")
}

func setLogfn(l *logp.Logger, level string) logfn {
	switch level {
	case "error":
		return l.Error
	case "warn":
		return l.Warn
	case "info":
		return l.Info
	case "debug":
		return l.Debug
	default:
		return l.Info
	}
}
