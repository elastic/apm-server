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

package modelprocessor

import (
	"context"
	"sync"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/common"
)

// IndexDocCountField is a modelprocessor that sets whether or not
// _doc_comment should be indexed on metricsets within a model.APMEvent.
type IndexDocCountField struct {
	mu sync.RWMutex
	// ElasticSearch cluster version
	version *common.Version
}

// SetESClusterVersion sets the elasticsearch cluster version, which is used to
// decide whether _doc_count should be indexed or not.
func (i *IndexDocCountField) SetESClusterVersion(version *common.Version) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.version = version
}

// Lowest version of elasticsearch that supports _doc_count being added by
// apm-server.
var elasticsearchSupportsDocCount = common.MustNewVersion("7.11.0")

// ProcessBatch sets whether the APMEvent should include _doc_comment on metricsets.
func (id *IndexDocCountField) ProcessBatch(ctx context.Context, b *model.Batch) error {
	id.mu.RLock()
	if id.version == nil {
		return nil
	}
	version := *(id.version)
	id.mu.RUnlock()
	for i := range *b {
		(&(*b)[i]).SetDocCount = !version.LessThan(elasticsearchSupportsDocCount)
	}
	return nil
}
