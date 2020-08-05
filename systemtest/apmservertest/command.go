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

package apmservertest

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// TODO(axw): add support for building/running the OSS apm-server.

// ServerCommand returns a ServerCmd (wrapping os/exec) for running
// apm-server with args.
func ServerCommand(subcommand string, args ...string) *ServerCmd {
	binary, buildErr := buildServer()
	if buildErr != nil {
		// Dummy command; Start etc. will return the build error.
		binary = "/usr/bin/false"
	}
	args = append([]string{subcommand}, args...)
	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = serverCommandSysProcAttr
	return &ServerCmd{
		Cmd:        cmd,
		buildError: buildErr,
	}
}

// ServerCmd wraps an os/exec.Cmd, taking care of building apm-server
// and cleaning up on close.
type ServerCmd struct {
	*exec.Cmd
	buildError error
	tempdir    string
}

// Run runs the apm-server command, and waits for it to exit.
func (c *ServerCmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Output runs the apm-server command, waiting for it to exit
// and returning its stdout.
func (c *ServerCmd) Output() ([]byte, error) {
	if err := c.prestart(); err != nil {
		return nil, err
	}
	defer c.cleanup()
	return c.Cmd.Output()
}

// CombinedOutput runs the apm-server command, waiting for it to exit
// and returning its combined stdout/stderr.
func (c *ServerCmd) CombinedOutput() ([]byte, error) {
	if err := c.prestart(); err != nil {
		return nil, err
	}
	defer c.cleanup()
	return c.Cmd.CombinedOutput()
}

// Start starts the apm-server command, and returns immediately.
func (c *ServerCmd) Start() error {
	if err := c.prestart(); err != nil {
		return err
	}
	if err := c.Cmd.Start(); err != nil {
		c.cleanup()
		return err
	}
	return nil
}

// Wait waits for the previously started apm-server command to exit.
func (c *ServerCmd) Wait() error {
	defer c.cleanup()
	return c.Cmd.Wait()
}

func (c *ServerCmd) prestart() error {
	if c.buildError != nil {
		return c.buildError
	}
	if c.Dir == "" {
		if err := c.createTempDir(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ServerCmd) createTempDir() error {
	tempdir, err := ioutil.TempDir("", "apm-server-systemtest")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(tempdir, "apm-server.yml"), nil, 0644); err != nil {
		os.RemoveAll(tempdir)
		return err
	}

	// Symlink ingest/pipeline/definition.json into the temporary directory.
	pipelineDir := filepath.Join(tempdir, "ingest", "pipeline")
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		os.RemoveAll(tempdir)
		return err
	}
	pipelineDefinitionFile := filepath.Join(filepath.Dir(c.Cmd.Path), "ingest", "pipeline", "definition.json")
	pipelineDefinitionSymlink := filepath.Join(pipelineDir, "definition.json")
	if err := os.Symlink(pipelineDefinitionFile, pipelineDefinitionSymlink); err != nil {
		if !os.IsExist(err) {
			os.RemoveAll(tempdir)
			return err
		}
	}

	c.tempdir = tempdir
	c.Dir = tempdir
	return nil
}

func (c *ServerCmd) cleanup() {
	if c.tempdir != "" {
		os.RemoveAll(c.tempdir)
	}
}

// buildServer builds the apm-server binary, returning its absolute path.
func buildServer() (string, error) {
	apmServerBinaryMu.Lock()
	defer apmServerBinaryMu.Unlock()
	if apmServerBinary != "" {
		return apmServerBinary, nil
	}

	// Build apm-server binary in the repo root.
	output, err := exec.Command("go", "list", "-m", "-f={{.Dir}}/..").Output()
	if err != nil {
		return "", err
	}
	repoRoot := filepath.Clean(strings.TrimSpace(string(output)))
	abspath := filepath.Join(repoRoot, "apm-server")
	if runtime.GOOS == "windows" {
		abspath += ".exe"
	}

	log.Println("Building apm-server...")
	cmd := exec.Command("go", "build", "-o", abspath, "./x-pack/apm-server")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	log.Println("Built", abspath)
	apmServerBinary = abspath
	return apmServerBinary, nil
}

var (
	apmServerBinaryMu sync.Mutex
	apmServerBinary   string
)
