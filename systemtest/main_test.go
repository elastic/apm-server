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
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestMain(m *testing.M) {
	var errg errgroup.Group

	errg.Go(func() error {
		log.Println("INFO: starting stack containers...")
		if err := StartStackContainers(); err != nil {
			return fmt.Errorf("failed to start stack containers: %w", err)
		}
		return nil
	})

	errg.Go(func() error {
		log.Println("INFO: building the integration package...")
		if err := buildIntegrationPackage(); err != nil {
			return fmt.Errorf("failed to build the integration package: %w", err)
		}

		return nil
	})

	if err := errg.Wait(); err != nil {
		log.Fatal(err)
	}

	errg.Go(func() error {
		log.Println("INFO: cleaning up Elasticsearch...")
		if err := cleanupElasticsearch(); err != nil {
			return fmt.Errorf("failed to cleanup Elasticsearch: %w", err)
		}

		return nil
	})

	errg.Go(func() error {
		log.Println("INFO: setting up fleet...")
		if err := InitFleet(); err != nil {
			return fmt.Errorf("failed to setup fleet: %w", err)
		}

		return nil
	})

	if err := errg.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Println("INFO: running system tests...")
	os.Exit(m.Run())
}

func buildIntegrationPackage() error {
	// Build the integration package.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("could not locate systemtest directory")
	}
	systemtestDir := filepath.Dir(filename)
	repoRoot := filepath.Join(systemtestDir, "..")
	cmd := exec.Command("make", "build-package")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
