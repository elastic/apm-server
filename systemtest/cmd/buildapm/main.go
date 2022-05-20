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

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/elastic/apm-server/systemtest"
)

var (
	arch       string
	cloudImage bool
)

func init() {
	flag.StringVar(&arch, "arch", `amd64`, "The architecture to use for the APM Server and Docker Image")
	flag.BoolVar(&cloudImage, "cloud", false, "Toggle to the Elastic Agent cloud image as the base")
	rand.Seed(time.Now().Unix())
}

type compose struct {
	Services struct {
		FleetServer struct {
			Image string `yaml:"image"`
		} `yaml:"fleet-server"`
	} `yaml:"services"`
}

func Main() error {
	flag.Parse()

	f, err := os.Open(filepath.Join("..", "..", "..", "docker-compose.yml"))
	if err != nil {
		return err
	}
	var c compose
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return err
	}
	image := c.Services.FleetServer.Image
	i := strings.LastIndex(image, ":")
	if i < 0 {
		return fmt.Errorf("invalid fleet-server image: %s", image)
	}
	tag := image[i+1:]
	opts := systemtest.ContainerConfig{
		Arch:             arch,
		BaseImageVersion: tag,
	}
	if cloudImage {
		opts.BaseImage = "docker.elastic.co/cloud-release/elastic-agent-cloud"
	}

	agent, err := systemtest.NewUnstartedElasticAgentContainer(opts)
	if err != nil {
		return err
	}
	return agent.Close()
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
