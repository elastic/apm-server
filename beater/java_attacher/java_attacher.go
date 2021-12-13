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
	cfg    config.JavaAttacherConfig
	logger *logp.Logger
}

func New(cfg config.JavaAttacherConfig) (JavaAttacher, error) {
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
	if _, err := os.Stat(javaAttacher); err != nil {
		return JavaAttacher{}, err
	}
	return JavaAttacher{
		cfg:    cfg,
		logger: logger,
	}, nil
}

// javaAttacher is bundled by the server
var javaAttacher = filepath.FromSlash("./java-attacher.jar")

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
				j.logger.Debug(b.Message)
			default:
				j.logger.Errorf("unrecognized java-attacher log.level: %s", b.LogLevel)
			}
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
	args := []string{"--continuous", "--log-level", "debug"}

	if j.cfg.DownloadAgentVersion != "" {
		args = append(args, "--download-agent-version", j.cfg.DownloadAgentVersion)
	}

	for _, flag := range j.cfg.DiscoveryRules {
		for name, value := range flag {
			if strings.HasSuffix(name, "vmarg") {
				// TODO(stn): the bundled version of
				// java-attacher, 1.27.0, only supports
				// `-vmargs`. This will be changed to `-vmarg`
				// in the future. For consistency in naming and
				// with other projects, expose `-vmarg` to the
				// user, but internally convert it. This should
				// be removed once the bundled java-attacher
				// version supports `-vmarg`.
				name += "s"
			}
			args = append(args, "--"+name, value)
		}
	}
	cfg := make([]string, 0, len(j.cfg.Config))
	for k, v := range j.cfg.Config {
		cfg = append(cfg, "--config", k+"="+v)
	}

	return append(args, cfg...)
}
