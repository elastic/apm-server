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

package licenser

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs/elasticsearch"
)

// Enforce setups the corresponding callbacks in libbeat to verify the license on the
// remote elasticsearch cluster.
func Enforce(log *logp.Logger, name string, checks ...CheckFunc) {
	cb := func(client *elasticsearch.Client) error {
		fetcher := NewElasticFetcher(client)
		license, err := fetcher.Fetch()

		if err != nil {
			return errors.Wrapf(err, "cannot retrieve the elasticsearch license")
		}

		if license == OSSLicense {
			return errors.Errorf("%s requires the default distribution of Elasticsearch. Please "+
				"update to the default distribution of Elasticsearch for full access to all "+
				"free features, or switch to the OSS distribution of %s.", name, name)
		}

		if !Validate(log, *license, checks...) {
			return fmt.Errorf(
				"invalid license found, requires a basic or a valid trial license and received %s",
				license.Get(),
			)
		}

		return nil
	}

	elasticsearch.RegisterGlobalCallback(cb)
}
