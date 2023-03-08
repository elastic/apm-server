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
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/docker/docker/client"

	"github.com/elastic/apm-server/systemtest"
)

var (
	arch             string
	outputName       string
	cloudImage       bool
	baseImageVersion string
)

func init() {
	flag.StringVar(&arch, "arch", `amd64`, "The architecture to use for the APM Server and Docker Image")
	flag.StringVar(&outputName, "name", "", "The name of the output Docker image")
	flag.StringVar(&baseImageVersion, "tag", "", "The tag of the Elastic Agent base image")
	flag.BoolVar(&cloudImage, "cloud", false, "Toggle to the Elastic Agent cloud image as the base")
	rand.Seed(time.Now().Unix())
}

func Main() error {
	flag.Parse()

	if baseImageVersion == "" {
		// Locate docker-compose.yml at the root of the repo.
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			return errors.New("failed to locate source directory")
		}
		repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
		data, err := os.ReadFile(filepath.Join(repoRoot, "docker-compose.yml"))
		if err != nil {
			return err
		}
		imageRegexp := regexp.MustCompile("docker.elastic.co/.*:(.*)")
		match := imageRegexp.FindStringSubmatch(string(data))
		if match == nil {
			return errors.New("failed to locate stack version")
		}
		baseImageVersion = match[1]
		log.Printf("Found stack version %q", baseImageVersion)
	}

	baseImage := systemtest.ElasticAgentImage
	if cloudImage {
		baseImage = "docker.elastic.co/cloud-release/elastic-agent-cloud"
	}

	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer docker.Close()
	docker.NegotiateAPIVersion(context.Background())

	if outputName == "" {
		outputName = fmt.Sprintf("elastic-agent-systemtest:%s", baseImageVersion)
	}
	return systemtest.BuildElasticAgentImage(
		context.Background(),
		docker, arch,
		fmt.Sprintf("%s:%s", baseImage, baseImageVersion),
		outputName,
	)
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
