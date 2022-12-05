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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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
)

const (
	startContainersTimeout = 5 * time.Minute

	ElasticAgentImage = "docker.elastic.co/beats/elastic-agent"
)

var (
	containerReaper         *testcontainers.Reaper
	initContainerReaperOnce sync.Once

	systemtestDir string
)

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("could not locate systemtest directory")
	}
	systemtestDir = filepath.Dir(filename)
}

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

type ContainerConfig struct {
	Name string
	Arch string

	// BaseImage, if non-empty, specifies the elastic-agent
	// image to use. If BaseImage is empty, ElasticAgentImage
	// will be used.
	BaseImage string

	// BaseImageVersion, if non-empty, specifies the elastic-agent
	// image tag to use. If BaseImageVersion is empty, the image
	// tag used by the fleet-server docker-compose service will be
	// used.
	BaseImageVersion string
}

// NewUnstartedElasticAgentContainer returns a new ElasticAgentContainer.
func NewUnstartedElasticAgentContainer(opts ContainerConfig) (*ElasticAgentContainer, error) {
	// Create a testcontainer.ContainerRequest to run Elastic Agent.
	// We pull some configuration from the Kibana docker-compose service,
	// such as the Docker network to use.
	if opts.Arch == "" {
		opts.Arch = runtime.GOARCH
	}
	if opts.BaseImage == "" {
		opts.BaseImage = ElasticAgentImage
	}

	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	var networks []string
	if opts.BaseImageVersion == "" {
		fleetServerContainer, err := stackContainerInfo(context.Background(), docker, "fleet-server")
		if err != nil {
			return nil, err
		}
		fleetServerContainerDetails, err := docker.ContainerInspect(context.Background(), fleetServerContainer.ID)
		if err != nil {
			return nil, err
		}

		for network := range fleetServerContainerDetails.NetworkSettings.Networks {
			networks = append(networks, network)
		}

		// Use the same elastic-agent image as used for fleet-server.
		opts.BaseImageVersion = fleetServerContainer.Image[strings.LastIndex(fleetServerContainer.Image, ":")+1:]
	}

	// Build a custom elastic-agent image with a locally built apm-server binary injected.
	agentImage := fmt.Sprintf("elastic-agent-systemtest:%s", opts.BaseImageVersion)
	if err := BuildElasticAgentImage(context.Background(),
		docker, opts.Arch,
		fmt.Sprintf("%s:%s", opts.BaseImage, opts.BaseImageVersion),
		agentImage,
	); err != nil {
		return nil, err
	}

	agentImageDetails, _, err := docker.ImageInspectWithRaw(context.Background(), agentImage)
	if err != nil {
		return nil, err
	}
	vcsRef := agentImageDetails.Config.Labels["org.label-schema.vcs-ref"]

	containerCACertPath := "/etc/pki/tls/certs/fleet-ca.pem"
	hostCACertPath := filepath.Join(systemtestDir, "../testing/docker/fleet-server/ca.pem")
	req := testcontainers.ContainerRequest{
		Name:       opts.Name,
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
		vcsRef:  vcsRef,
		request: req,
		exited:  make(chan struct{}),
		Reap:    true,
	}, nil
}

// ElasticAgentContainer represents an ephemeral Elastic Agent container.
type ElasticAgentContainer struct {
	vcsRef    string
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

// APMServerLog returns the contents of the APM Sever log file.
func (c *ElasticAgentContainer) APMServerLog() (io.ReadCloser, error) {
	return c.container.CopyFileFromContainer(
		context.Background(), fmt.Sprintf(
			"/usr/share/elastic-agent/state/data/logs/apm-default-%s-1.ndjson",
			time.Now().UTC().Format("20060102"),
		),
	)
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

// BuildElasticAgentImage builds a Docker image from the published image with a locally built apm-server injected.
func BuildElasticAgentImage(
	ctx context.Context,
	docker *client.Client,
	arch string,
	baseImage, outputImageName string,
) error {
	agentImageMu.Lock()
	defer agentImageMu.Unlock()
	if agentImages[arch] {
		return nil
	}

	log.Printf("Building image %s (%s) from %s...", outputImageName, arch, baseImage)
	cmd := exec.Command(
		"bash",
		filepath.Join(systemtestDir, "..", "testing", "docker", "elastic-agent", "build.sh"),
		"-t", outputImageName,
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "BASE_IMAGE="+baseImage, "GOARCH="+arch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	log.Printf("Built image %s (%s)", outputImageName, arch)
	agentImages[arch] = true
	return nil
}

var (
	agentImageMu sync.RWMutex
	agentImages  = make(map[string]bool)
)
