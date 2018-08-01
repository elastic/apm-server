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

package transaction

import (
	"github.com/elastic/apm-server/transform"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	cachedSchema = validation.CreateSchema(schema.PayloadSchema, "transaction")
)

func PayloadSchema() *jsonschema.Schema {
	return cachedSchema
}

func DecodePayload(raw map[string]interface{}) ([]transform.Eventable, error) {
	if raw == nil {
		return nil, nil
	}

	decoder := utility.ManualDecoder{}
	txs := decoder.InterfaceArr(raw, "transactions")
	err := decoder.Err
	events := make([]transform.Eventable, len(txs))
	for idx, tx := range txs {
		events[idx], err = DecodeEvent(tx, err)
	}
	return events, err
}
