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

package systemtest

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/gofrs/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v8"
)

const (
	startContainersTimeout = 5 * time.Minute
)

var (
	containerReaper         *testcontainers.Reaper
	initContainerReaperOnce sync.Once
)

// InitContainerReaper initialises the testcontainers container reaper,
// which will ensure all containers started by testcontainers are removed
// after some time if they are left running when the systemtest process
// exits.
func initContainerReaper() {
	dockerProvider, err := testcontainers.NewDockerProvider()
	if err != nil {
		panic(err)
	}

	sessionUUID := uuid.Must(uuid.NewV4())
	containerReaper, err = testcontainers.NewReaper(
		context.Background(),
		sessionUUID.String(),
		dockerProvider,
		testcontainers.ReaperDefaultImage,
	)
	if err != nil {
		panic(err)
	}

	// The connection will be closed on exit.
	if _, err = containerReaper.Connect(); err != nil {
		panic(err)
	}
}

// StartStackContainers starts Docker containers for Elasticsearch and Kibana.
//
// We leave Elasticsearch and Kibana running, to avoid slowing down iterative
// development and testing. Use docker-compose to stop services as necessary.
func StartStackContainers() error {
	cmd := exec.Command(
		"docker-compose", "-f", "../docker-compose.yml",
		"up", "-d", "elasticsearch", "kibana", "fleet-server",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Wait for up to 5 minutes for Kibana and Fleet Server to become healthy,
	// which implies Elasticsearch is healthy too.
	ctx, cancel := context.WithTimeout(context.Background(), startContainersTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return waitContainerHealthy(ctx, "kibana") })
	g.Go(func() error { return waitContainerHealthy(ctx, "fleet-server") })
	return g.Wait()
}

// NewUnstartedElasticsearchContainer returns a new ElasticsearchContainer.
func NewUnstartedElasticsearchContainer() (*ElasticsearchContainer, error) {
	// Create a testcontainer.ContainerRequest based on the "elasticsearch service
	// defined in docker-compose.

	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	container, err := stackContainerInfo(context.Background(), docker, "elasticsearch")
	if err != nil {
		return nil, err
	}
	containerDetails, err := docker.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		Image:      container.Image,
		AutoRemove: true,
		SkipReaper: true, // we use our own reaping logic
	}
	req.WaitingFor = wait.ForHTTP("/").WithPort("9200/tcp")

	initContainerReaperOnce.Do(initContainerReaper)
	req.Labels = make(map[string]string)
	for k, v := range containerReaper.Labels() {
		req.Labels[k] = v
	}

	for port := range containerDetails.Config.ExposedPorts {
		req.ExposedPorts = append(req.ExposedPorts, string(port))
	}

	env := make(map[string]string)
	for _, kv := range containerDetails.Config.Env {
		sep := strings.IndexRune(kv, '=')
		k, v := kv[:sep], kv[sep+1:]
		env[k] = v
	}
	for network := range containerDetails.NetworkSettings.Networks {
		req.Networks = append(req.Networks, network)
	}

	// BUG(axw) ElasticsearchContainer currently does not support security.
	env["xpack.security.enabled"] = "false"

	return &ElasticsearchContainer{request: req, Env: env}, nil
}

func waitContainerHealthy(ctx context.Context, serviceName string) error {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer docker.Close()

	container, err := stackContainerInfo(ctx, docker, serviceName)
	if err != nil {
		return err
	}

	first := true
	for {
		containerJSON, err := docker.ContainerInspect(ctx, container.ID)
		if err != nil {
			return err
		}
		if containerJSON.State.Health.Status == "healthy" {
			break
		}
		if first {
			log.Printf("Waiting for %s container (%s) to become healthy", serviceName, container.ID)
			first = false
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

func stackContainerInfo(ctx context.Context, docker *client.Client, name string) (*types.Container, error) {
	containers, err := docker.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project=apm-server"),
			filters.Arg("label", "com.docker.compose.service="+name),
		),
	})
	if err != nil {
		return nil, err
	}
	if n := len(containers); n != 1 {
		return nil, fmt.Errorf("expected 1 %s container, got %d", name, n)
	}
	return &containers[0], nil
}

// ElasticsearchContainer represents an ephemeral Elasticsearch container.
//
// This can be used when the docker-compose "elasticsearch" service is insufficient.
type ElasticsearchContainer struct {
	request   testcontainers.ContainerRequest
	container testcontainers.Container

	// Env holds the environment variables to pass to the container,
	// and will be initialised with the values in the docker-compose
	// "elasticsearch" service definition.
	//
	// BUG(axw) ElasticsearchContainer currently does not support security,
	// and will set "xpack.security.enabled=false" by default.
	Env map[string]string

	// Addr holds the "host:port" address for Elasticsearch's REST API.
	// This will be populated by Start.
	Addr string

	// Client holds a client for interacting with Elasticsearch's REST API.
	// This will be populated by Start.
	Client *estest.Client
}

// Start starts the container.
//
// The Addr and Client fields will be updated on successful return.
//
// The container will be removed when Close() is called, or otherwise by a
// reaper process if the test process is aborted.
func (c *ElasticsearchContainer) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), startContainersTimeout)
	defer cancel()

	// Update request from user-definable fields.
	c.request.Env = c.Env

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: c.request,
	})
	if err != nil {
		return err
	}
	c.container = container

	if err := c.container.Start(ctx); err != nil {
		return err
	}
	ip, err := container.Host(ctx)
	if err != nil {
		return err
	}
	port, err := container.MappedPort(ctx, "9200")
	if err != nil {
		return err
	}
	c.Addr = net.JoinHostPort(ip, port.Port())

	esURL := url.URL{Scheme: "http", Host: c.Addr}
	config := newElasticsearchConfig()
	config.Addresses[0] = esURL.String()
	client, err := elasticsearch.NewClient(config)
	if err != nil {
		return err
	}
	c.Client = &estest.Client{Client: client}

	c.container = container
	return nil
}

// Close terminates and removes the container.
func (c *ElasticsearchContainer) Close() error {
	if c.container == nil {
		return nil
	}
	return c.container.Terminate(context.Background())
}

// NewUnstartedElasticAgentContainer returns a new ElasticAgentContainer.
func NewUnstartedElasticAgentContainer() (*ElasticAgentContainer, error) {
	// Create a testcontainer.ContainerRequest to run Elastic Agent.
	// We pull some configuration from the Kibana docker-compose service,
	// such as the Docker network to use.

	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	fleetServerContainer, err := stackContainerInfo(context.Background(), docker, "fleet-server")
	if err != nil {
		return nil, err
	}
	fleetServerContainerDetails, err := docker.ContainerInspect(context.Background(), fleetServerContainer.ID)
	if err != nil {
		return nil, err
	}

	var networks []string
	for network := range fleetServerContainerDetails.NetworkSettings.Networks {
		networks = append(networks, network)
	}
	containerCACertPath := "/etc/pki/tls/certs/fleet-ca.pem"

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("could not locate systemtest directory")
	}
	systemtestDir := filepath.Dir(filename)
	hostCACertPath := filepath.Join(systemtestDir, "../testing/docker/fleet-server/ca.pem")

	// Use the same elastic-agent image as used for fleet-server.
	agentImageVersion := fleetServerContainer.Image[strings.LastIndex(fleetServerContainer.Image, ":")+1:]
	agentImage := "docker.elastic.co/beats/elastic-agent:" + agentImageVersion
	agentImageDetails, _, err := docker.ImageInspectWithRaw(context.Background(), agentImage)
	if err != nil {
		return nil, err
	}
	stackVersion := agentImageDetails.Config.Labels["org.label-schema.version"]
	vcsRef := agentImageDetails.Config.Labels["org.label-schema.vcs-ref"]

	// Build a custom elastic-agent image with a locally built apm-server binary injected.
	agentImage, err = buildElasticAgentImage(context.Background(), docker, stackVersion, agentImageVersion, vcsRef)
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		Image:      agentImage,
		AutoRemove: true,
		Networks:   networks,
		BindMounts: map[string]string{hostCACertPath: containerCACertPath},
		Env: map[string]string{
			"FLEET_URL": "https://fleet-server:8220",
			"FLEET_CA":  containerCACertPath,
		},
		SkipReaper: true, // we use our own reaping logic
	}
	return &ElasticAgentContainer{
		request: req,
		exited:  make(chan struct{}),
		Reap:    true,
	}, nil
}

// ElasticAgentContainer represents an ephemeral Elastic Agent container.
type ElasticAgentContainer struct {
	container testcontainers.Container
	request   testcontainers.ContainerRequest
	exited    chan struct{}

	// Reap entrols whether the container will be automatically reaped if
	// the controlling process exits. This is true by default, and may be
	// set to false before the container is started to prevent the container
	// from being stoped and removed.
	Reap bool

	// ExposedPorts holds an optional list of ports to expose to the host.
	ExposedPorts []string

	// WaitingFor holds an optional wait strategy.
	WaitingFor wait.Strategy

	// Addrs holds the "host:port" address for each exposed port, mapped
	// by exposed port. This will be populated by Start.
	Addrs map[string]string

	// FleetEnrollmentToken holds an optional Fleet enrollment token to
	// use for enrolling the agent with Fleet. The agent will only enroll
	// if this is specified.
	FleetEnrollmentToken string

	// Stdout, if non-nil, holds a writer to which the container's stdout
	// will be written.
	Stdout io.Writer

	// Stderr, if non-nil, holds a writer to which the container's stderr
	// will be written.
	Stderr io.Writer
}

// Start starts the container.
//
// The Addr and Client fields will be updated on successful return.
//
// The container will be removed when Close() is called, or otherwise by a
// reaper process if the test process is aborted.
func (c *ElasticAgentContainer) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), startContainersTimeout)
	defer cancel()

	// Update request from user-definable fields.
	if c.FleetEnrollmentToken != "" {
		c.request.Env["FLEET_ENROLL"] = "1"
		c.request.Env["FLEET_ENROLLMENT_TOKEN"] = c.FleetEnrollmentToken
	}
	c.request.ExposedPorts = c.ExposedPorts
	c.request.WaitingFor = c.WaitingFor
	if c.Reap {
		initContainerReaperOnce.Do(initContainerReaper)
		c.request.Labels = make(map[string]string)
		for k, v := range containerReaper.Labels() {
			c.request.Labels[k] = v
		}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: c.request,
	})
	if err != nil {
		return err
	}
	c.container = container

	// Start a goroutine to read logs, and signal when the container process has exited.
	if c.Stdout != nil || c.Stderr != nil {
		go func() {
			defer close(c.exited)
			defer cancel()
			stdout, stderr := c.Stdout, c.Stderr
			if stdout == nil {
				stdout = io.Discard
			}
			if stderr == nil {
				stderr = io.Discard
			}
			_ = c.copyLogs(stdout, stderr)
		}()
	}

	if err := container.Start(ctx); err != nil {
		if err != context.Canceled {
			return fmt.Errorf("failed to start container: %w", err)
		}
		return errors.New("failed to start container")
	}

	if len(c.request.ExposedPorts) > 0 {
		hostIP, err := container.Host(ctx)
		if err != nil {
			return err
		}
		c.Addrs = make(map[string]string)
		for _, exposedPort := range c.request.ExposedPorts {
			mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
			if err != nil {
				return err
			}
			c.Addrs[exposedPort] = net.JoinHostPort(hostIP, mappedPort.Port())
		}
	}

	c.container = container
	return nil
}

func (c *ElasticAgentContainer) copyLogs(stdout, stderr io.Writer) error {
	// Wait for the container to be running (or have gone past that),
	// or ContainerLogs will return immediately.
	ctx := context.Background()
	for {
		state, err := c.container.State(ctx)
		if err != nil {
			return err
		}
		if state.Status != "created" {
			break
		}
	}

	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer docker.Close()

	options := types.ContainerLogsOptions{
		ShowStdout: stdout != nil,
		ShowStderr: stderr != nil,
		Follow:     true,
	}
	rc, err := docker.ContainerLogs(ctx, c.container.GetContainerID(), options)
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = stdcopy.StdCopy(stdout, stderr, rc)
	return err
}

// Close terminates and removes the container.
func (c *ElasticAgentContainer) Close() error {
	if c.container == nil {
		return nil
	}
	return c.container.Terminate(context.Background())
}

// Wait waits for the container process to exit, and returns its state.
func (c *ElasticAgentContainer) Wait(ctx context.Context) (*types.ContainerState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.exited:
		return c.container.State(ctx)
	}
}

// Exec executes a command in the container, and returns its stdout and stderr.
func (c *ElasticAgentContainer) Exec(ctx context.Context, cmd ...string) (stdout, stderr []byte, _ error) {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, nil, err
	}
	defer docker.Close()

	response, err := docker.ContainerExecCreate(ctx, c.container.GetContainerID(), types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          cmd,
	})
	if err != nil {
		return nil, nil, err
	}

	// Consume all the exec output.
	resp, err := docker.ContainerExecAttach(ctx, response.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, nil, err
	}
	defer resp.Close()
	var stdoutBuf, stderrBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, resp.Reader); err != nil {
		return nil, nil, err
	}

	// Return an error if the command exited non-zero.
	execResp, err := docker.ContainerExecInspect(ctx, response.ID)
	if err != nil {
		return nil, nil, err
	}
	if execResp.ExitCode != 0 {
		return nil, nil, fmt.Errorf("process exited with code %d", execResp.ExitCode)
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}

func matchFleetServerAPIStatusHealthy(r io.Reader) bool {
	var status struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(r).Decode(&status); err != nil {
		return false
	}
	return status.Status == "HEALTHY"
}

// buildElasticAgentImage builds a Docker image from the published image with a locally built apm-server injected.
func buildElasticAgentImage(ctx context.Context, docker *client.Client, stackVersion, imageVersion, vcsRef string) (string, error) {
	imageName := fmt.Sprintf("elastic-agent-systemtest:%s", imageVersion)
	log.Printf("Building image %s...", imageName)

	// Build apm-server, and copy it into the elastic-agent container's "install" directory.
	// This bypasses downloading the artifact.
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	vcsRefShort := vcsRef[:6]
	apmServerInstallDir := fmt.Sprintf("./data/elastic-agent-%s/install/apm-server-%s-linux-%s", vcsRefShort, stackVersion, arch)
	apmServerBinary, err := apmservertest.BuildServerBinary("linux")
	if err != nil {
		return "", err
	}

	// Binaries to copy from disk into the build context.
	binaries := map[string]string{
		"apm-server": apmServerBinary,
	}

	// Generate Dockerfile contents.
	var dockerfile bytes.Buffer
	fmt.Fprintf(&dockerfile, "FROM docker.elastic.co/beats/elastic-agent:%s\n", imageVersion)
	fmt.Fprintf(&dockerfile, "COPY --chown=elastic-agent:elastic-agent apm-server apm-server.yml %s/\n", apmServerInstallDir)

	// Files to generate in the build context.
	generatedFiles := map[string][]byte{
		"Dockerfile":     dockerfile.Bytes(),
		"apm-server.yml": []byte(""),
	}

	var buildContext bytes.Buffer
	tarw := tar.NewWriter(&buildContext)
	for name, path := range binaries {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return "", err
		}
		if err := tarw.WriteHeader(&tar.Header{
			Name:  name,
			Size:  info.Size(),
			Mode:  0755,
			Uname: "elastic-agent",
			Gname: "elastic-agent",
		}); err != nil {
			return "", err
		}
		if _, err := io.Copy(tarw, f); err != nil {
			return "", err
		}
	}
	for name, content := range generatedFiles {
		if err := tarw.WriteHeader(&tar.Header{
			Name:  name,
			Size:  int64(len(content)),
			Mode:  0644,
			Uname: "elastic-agent",
			Gname: "elastic-agent",
		}); err != nil {
			return "", err
		}
		if _, err := tarw.Write(content); err != nil {
			return "", err
		}
	}
	if err := tarw.Close(); err != nil {
		return "", err
	}

	resp, err := docker.ImageBuild(ctx, &buildContext, types.ImageBuildOptions{Tags: []string{imageName}})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
		return "", err
	}
	log.Printf("Built image %s", imageName)
	return imageName, nil
}
