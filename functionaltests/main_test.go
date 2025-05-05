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

package functionaltests

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

func TestMain(m *testing.M) {
	flag.Parse()

	// This is a simple check to alert users if this necessary env var
	// is not available.
	//
	// Functional tests are expected to run Terraform code to operate
	// on infrastructure required for each test and to query Elastic
	// Cloud APIs. In both cases a valid API key is required.
	ecAPIKey := os.Getenv("EC_API_KEY")
	if ecAPIKey == "" {
		log.Fatal("EC_API_KEY env var not set")
		return
	}

	ctx := context.Background()
	ecRegion := regionFrom(*target)
	ecc, err := ecclient.New(endpointFrom(*target), ecAPIKey)
	if err != nil {
		log.Fatal(err)
		return
	}

	candidates, err := ecc.GetCandidateVersionInfos(ctx, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}
	fetchedCandidates = candidates

	snapshots, err := ecc.GetSnapshotVersionInfos(ctx, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}
	fetchedSnapshots = snapshots

	versions, err := ecc.GetVersionInfos(ctx, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}
	fetchedVersions = versions

	code := m.Run()
	os.Exit(code)
}
