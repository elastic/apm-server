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
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v7"
)

const (
	startContainersTimeout = 5 * time.Minute

	defaultFleetServerPort = "8220"
)

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
	req.WaitingFor = wait.ForHTTP("/").WithPort("9200/tcp")

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

	esContainer, err := stackContainerInfo(context.Background(), docker, "elasticsearch")
	if err != nil {
		return nil, err
	}
	esContainerDetails, err := docker.ContainerInspect(context.Background(), esContainer.ID)
	if err != nil {
		return nil, err
	}

	var esIPAddress string
	var networks []string
	for network, settings := range esContainerDetails.NetworkSettings.Networks {
		networks = append(networks, network)
		if esIPAddress == "" && settings.IPAddress != "" {
			esIPAddress = settings.IPAddress
		}
	}
	esURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(esIPAddress, apmservertest.ElasticsearchPort()),
	}

	// Use the same stack version as used for Elasticsearch.
	agentImageVersion := esContainer.Image[strings.LastIndex(esContainer.Image, ":")+1:]
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

	req := testcontainers.ContainerRequest{
		Image:      agentImage,
		AutoRemove: true,
		Networks:   networks,
		Env: map[string]string{
			// NOTE(axw) because we bind-mount the apm-server artifacts in, they end up owned by the
			// current user rather than root. Disable Beats's strict permission checks to avoid resulting
			// complaints, as they're irrelevant to these system tests.
			"BEAT_STRICT_PERMS": "false",
		},
	}
	return &ElasticAgentContainer{
		request:          req,
		installDir:       agentInstallDir,
		elasticsearchURL: esURL.String(),
		StackVersion:     agentImageVersion,
		BindMountInstall: make(map[string]string),
	}, nil
}

// ElasticAgentContainer represents an ephemeral Elastic Agent container.
type ElasticAgentContainer struct {
	container        testcontainers.Container
	request          testcontainers.ContainerRequest
	elasticsearchURL string // used by Fleet Server only

	// installDir holds the location of the "install" directory inside
	// the Elastic Agent container.
	//
	// This will be set when the ElasticAgentContainer object is created,
	// and can be used to anticipate the location into which artifacts
	// can be bind-mounted.
	installDir string

	// FleetServer controls whether this Elastic Agent bootstraps
	// Fleet Server.
	FleetServer bool

	// StackVersion holds the stack version of the container image,
	// e.g. 8.0.0-SNAPSHOT.
	StackVersion string

	// ExposedPorts holds an optional list of ports to expose to the host.
	ExposedPorts []string

	// WaitingFor holds an optional wait strategy.
	WaitingFor wait.Strategy

	// FleetServerURL holds the Fleet Server URL to enroll into.
	//
	// This will be populated by Start if FleetServer is true, using
	// one of the container's network aliases.
	FleetServerURL string

	// Addrs holds the "host:port" address for each exposed port, mapped
	// by exposed port. This will be populated by Start.
	Addrs map[string]string

	// BindMountInstall holds a map of files to bind mount into the
	// container, mapping from the host location to target paths relative
	// to the install directory in the container.
	BindMountInstall map[string]string

	// FleetEnrollmentToken holds an optional Fleet enrollment token to
	// use for enrolling the agent with Fleet. The agent will only enroll
	// if this is specified.
	FleetEnrollmentToken string
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
	if c.FleetServer {
		c.request.Env["FLEET_SERVER_ENABLE"] = "1"
		c.request.Env["FLEET_SERVER_ELASTICSEARCH_HOST"] = c.elasticsearchURL
		c.request.Env["FLEET_SERVER_ELASTICSEARCH_USERNAME"] = adminElasticsearchUser
		c.request.Env["FLEET_SERVER_ELASTICSEARCH_PASSWORD"] = adminElasticsearchPass

		// Wait for API status to report healthy.
		c.request.WaitingFor = wait.ForHTTP("/api/status").
			WithPort(defaultFleetServerPort + "/tcp").
			WithTLS(true).WithAllowInsecure(true).
			WithResponseMatcher(matchFleetServerAPIStatusHealthy)
		c.request.ExposedPorts = []string{defaultFleetServerPort}
	} else if c.FleetEnrollmentToken != "" {
		c.request.Env["FLEET_ENROLL"] = "1"
		c.request.Env["FLEET_ENROLLMENT_TOKEN"] = c.FleetEnrollmentToken
		c.request.Env["FLEET_INSECURE"] = "1"
		c.request.Env["FLEET_URL"] = c.FleetServerURL
	}
	c.request.ExposedPorts = append(c.request.ExposedPorts, c.ExposedPorts...)
	if c.WaitingFor != nil {
		c.request.WaitingFor = c.WaitingFor
	}
	c.request.BindMounts = map[string]string{}
	for source, target := range c.BindMountInstall {
		c.request.BindMounts[source] = path.Join(c.installDir, target)
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: c.request,
	})
	if err != nil {
		return err
	}
	c.container = container

	if err := container.Start(ctx); err != nil {
		return err
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
	if c.FleetServer {
		networkAliases, err := container.NetworkAliases(ctx)
		if err != nil {
			return err
		}
		var networkAlias string
		for _, networkAliases := range networkAliases {
			if len(networkAliases) > 0 {
				networkAlias = networkAliases[0]
				break
			}
		}
		if networkAlias == "" {
			return errors.New("no network alias found")
		}
		fleetServerURL := &url.URL{Scheme: "https", Host: net.JoinHostPort(networkAlias, defaultFleetServerPort)}
		c.FleetServerURL = fleetServerURL.String()
	}

	c.container = container
	return nil
}

// Close terminates and removes the container.
func (c *ElasticAgentContainer) Close() error {
	if c.container == nil {
		return nil
	}
	return c.container.Terminate(context.Background())
}

// Logs returns an io.ReadCloser that can be used for reading the
// container's combined stdout/stderr log. If the container has not
// been created by Start(), Logs will return an error.
func (c *ElasticAgentContainer) Logs(ctx context.Context) (io.ReadCloser, error) {
	if c.container == nil {
		return nil, errors.New("container not created")
	}
	return c.container.Logs(ctx)
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
