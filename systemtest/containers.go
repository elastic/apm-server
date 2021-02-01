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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v7"
)

const startContainersTimeout = 5 * time.Minute

// StartStackContainers starts Docker containers for Elasticsearch and Kibana.
//
// We leave Elasticsearch and Kibana running, to avoid slowing down iterative
// development and testing. Use docker-compose to stop services as necessary.
func StartStackContainers() error {
	cmd := exec.Command(
		"docker-compose", "-f", "../docker-compose.yml",
		"up", "-d", "elasticsearch", "kibana",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Wait for up to 5 minutes for Kibana to become healthy,
	// which implies Elasticsearch is healthy too.
	ctx, cancel := context.WithTimeout(context.Background(), startContainersTimeout)
	defer cancel()
	return waitKibanaContainerHealthy(ctx)
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
	}
	waitFor := wait.ForHTTP("/")
	waitFor.Port = "9200/tcp"
	req.WaitingFor = waitFor

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

func waitKibanaContainerHealthy(ctx context.Context) error {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer docker.Close()

	container, err := stackContainerInfo(ctx, docker, "kibana")
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
			log.Printf("Waiting for Kibana container (%s) to become healthy", container.ID)
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
		Started:          true,
	})
	if err != nil {
		return err
	}
	defer func() {
		if c.container == nil {
			// Something has gone wrong.
			container.Terminate(ctx)
		}
	}()

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

	kibanaContainer, err := stackContainerInfo(context.Background(), docker, "kibana")
	if err != nil {
		return nil, err
	}
	kibanaContainerDetails, err := docker.ContainerInspect(context.Background(), kibanaContainer.ID)
	if err != nil {
		return nil, err
	}

	var kibanaIPAddress string
	var networks []string
	for network, settings := range kibanaContainerDetails.NetworkSettings.Networks {
		networks = append(networks, network)
		if kibanaIPAddress == "" && settings.IPAddress != "" {
			kibanaIPAddress = settings.IPAddress
		}
	}
	kibanaURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(kibanaIPAddress, apmservertest.KibanaPort()),
	}

	// Use the same stack version as used for Kibana.
	agentImageVersion := kibanaContainer.Image[strings.LastIndex(kibanaContainer.Image, ":")+1:]
	agentImage := "docker.elastic.co/beats/elastic-agent:" + agentImageVersion
	if err := pullDockerImage(context.Background(), docker, agentImage); err != nil {
		return nil, err
	}
	agentImageDetails, _, err := docker.ImageInspectWithRaw(context.Background(), agentImage)
	if err != nil {
		return nil, err
	}
	agentVCSRef := agentImageDetails.Config.Labels["org.label-schema.vcs-ref"]
	agentDataHashDir := path.Join("/usr/share/elastic-agent/data", "elastic-agent-"+agentVCSRef[:6])
	agentInstallDir := path.Join(agentDataHashDir, "install")

	// Build elastic-agent to replace the binary inside the container.
	//
	// This enables us to inject a locally built, unsigned, apm-server artifact.
	//
	// TODO(axw) once apm-server is bundled we can stop building a custom
	// elastic-agent. We can then inject a custom apm-server into the *install*
	// directory. We do that now, but we still need to inject a custom artifact
	// into the *downloads* directory as well, to prevent fetching from the
	// internet.
	agentBinary, err := buildElasticAgent(agentVCSRef)
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		Image:      agentImage,
		AutoRemove: true,
		Networks:   networks,
		Env: map[string]string{
			"FLEET_ENROLL":          "1",
			"FLEET_ENROLL_INSECURE": "1",
			"FLEET_SETUP":           "1",
			"KIBANA_HOST":           kibanaURL.String(),
			"KIBANA_USERNAME":       adminKibanaUser,
			"KIBANA_PASSWORD":       adminKibanaPass,

			// TODO(axw) remove once https://github.com/elastic/elastic-agent-client/issues/20 is fixed
			"GODEBUG": "x509ignoreCN=0",

			// NOTE(axw) because we bind-mount the apm-server artifacts in, they end up owned by the
			// current user rather than root. Disable Beats's strict permission checks to avoid resulting
			// complaints, as they're irrelevant to these system tests.
			"BEAT_STRICT_PERMS": "false",
		},
		BindMounts: map[string]string{
			agentBinary: path.Join(agentDataHashDir, "elastic-agent"),
		},
	}
	return &ElasticAgentContainer{
		request:          req,
		installDir:       agentInstallDir,
		StackVersion:     agentImageVersion,
		BindMountInstall: make(map[string]string),
	}, nil
}

// ElasticAgentContainer represents an ephemeral Elastic Agent container.
type ElasticAgentContainer struct {
	container testcontainers.Container
	request   testcontainers.ContainerRequest

	// installDir holds the location of the "install" directory inside
	// the Elastic Agent container.
	//
	// This will be set when the ElasticAgentContainer object is created,
	// and can be used to anticipate the location into which artifacts
	// can be bind-mounted.
	installDir string

	// StackVersion holds the stack version of the container image,
	// e.g. 8.0.0-SNAPSHOT.
	StackVersion string

	// ExposedPorts holds an optional list of ports to expose to the host.
	ExposedPorts []string

	// WaitingFor holds an optional wait strategy.
	WaitingFor wait.Strategy

	// Addrs holds the "host:port" address for each exposed port.
	// This will be populated by Start.
	Addrs []string

	// BindMountInstall holds a map of files to bind mount into the
	// container, mapping from the host location to target paths relative
	// to the install directory in the container.
	BindMountInstall map[string]string
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
	c.request.ExposedPorts = c.ExposedPorts
	c.request.WaitingFor = c.WaitingFor
	for source, target := range c.BindMountInstall {
		c.request.BindMounts[source] = path.Join(c.installDir, target)
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: c.request,
		Started:          true,
	})
	if err != nil {
		return err
	}
	defer func() {
		if c.container == nil {
			// Something has gone wrong.
			container.Terminate(ctx)
		}
	}()

	ports, err := container.Ports(ctx)
	if err != nil {
		return err
	}
	if len(ports) > 0 {
		ip, err := container.Host(ctx)
		if err != nil {
			return err
		}
		for _, portbindings := range ports {
			for _, pb := range portbindings {
				c.Addrs = append(c.Addrs, net.JoinHostPort(ip, pb.HostPort))
			}
		}
	}

	c.container = container
	return nil
}

// Close terminates and removes the container.
func (c *ElasticAgentContainer) Close() error {
	return c.container.Terminate(context.Background())
}

func pullDockerImage(ctx context.Context, docker *client.Client, imageRef string) error {
	rc, err := docker.ImagePull(context.Background(), imageRef, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(ioutil.Discard, rc)
	return err
}

// buildElasticAgent builds elastic-agent from the commit defined in go.mod,
// in development mode to enable injecting unsigned artifacts.
//
// The "commit" argumented passed in comes from the Docker image, and is set
// in the binary so that it has a consistent idea of the directory structure.
func buildElasticAgent(commit string) (string, error) {
	elasticAgentBinaryMu.Lock()
	defer elasticAgentBinaryMu.Unlock()
	if elasticAgentBinary != "" {
		return elasticAgentBinary, nil
	}

	// Build apm-server binary from the repo root, store it in the build dir.
	output, err := exec.Command("go", "list", "-m", "-f={{.Dir}}/..").Output()
	if err != nil {
		return "", err
	}
	repoRoot := filepath.Clean(strings.TrimSpace(string(output)))
	abspath := filepath.Join(repoRoot, "build", "elastic-agent-nopgp")

	log.Println("Building elastic-agent...")
	ldflags := "" +
		" -X github.com/elastic/beats/v7/x-pack/elastic-agent/pkg/release.snapshot=true" +
		" -X github.com/elastic/beats/v7/x-pack/elastic-agent/pkg/release.allowEmptyPgp=true" +
		" -X github.com/elastic/beats/v7/x-pack/elastic-agent/pkg/release.allowUpgrade=true" +
		" -X github.com/elastic/beats/v7/libbeat/version.commit=" + commit
	cmd := exec.Command("go", "build", "-o", abspath,
		"-ldflags", ldflags,
		"github.com/elastic/beats/v7/x-pack/elastic-agent",
	)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOOS=linux")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	log.Println("Built", abspath)
	elasticAgentBinary = abspath
	return elasticAgentBinary, nil
}

var (
	elasticAgentBinaryMu sync.Mutex
	elasticAgentBinary   string
)
