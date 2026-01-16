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

package integrationservertest

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

var (
	// cleanupOnFailure determines whether the created resources should be cleaned up on test failure.
	cleanupOnFailure = flag.Bool(
		"cleanup-on-failure",
		true,
		"Whether to run cleanup even if the test failed.",
	)

	// target is the Elastic Cloud environment to target with these test.
	// We use 'pro' for production as that is the key used to retrieve EC_API_KEY from secret storage.
	target = flag.String(
		"target",
		"pro",
		"The target environment where to run tests againts. Valid values are: qa, pro.",
	)

	upgradePath = flag.String(
		"upgrade-path",
		"",
		"Versions to be used in TestUpgrade in upgrade_test.go, separated by '->'",
	)
)

var vsCache *ech.VersionsCache

func TestMain(m *testing.M) {
	flag.Parse()

	// This is a simple check to alert users if this necessary env var
	// is not available.
	//
	// Integration server tests are expected to run Terraform code to operate on
	// infrastructure required for each test and to query Elastic Cloud APIs.
	// In both cases a valid API key is required.
	ecAPIKey := os.Getenv("EC_API_KEY")
	if ecAPIKey == "" {
		log.Fatal("EC_API_KEY env var not set")
	}

	ctx := context.Background()
	ecRegion := RegionFrom(*target)
	ecc, err := ech.NewClient(EndpointFrom(*target), ecAPIKey)
	if err != nil {
		log.Fatal(err)
	}

	vsCache, err = ech.NewVersionsCache(ctx, ecc, ecRegion)
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	os.Exit(code)
}
