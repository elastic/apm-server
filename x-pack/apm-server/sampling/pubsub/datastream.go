// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/elastic/apm-server/elasticsearch"

	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// SetupDataStream ensures that the sampled traces data stream index template
// exists with the given name, creating it and its associated ILM policy if it
// does not.
//
// This should only be called when not running under Fleet.
func SetupDataStream(
	ctx context.Context,
	client elasticsearch.Client,
	indexTemplateName string,
	ilmPolicyName string,
	indexPattern string,
) error {
	// Create/replace ILM policy.
	resp, err := esapi.ILMPutLifecycleRequest{
		Policy: ilmPolicyName,
		Body:   strings.NewReader(ilmPolicy),
	}.Do(ctx, client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to create ILM policy (%d): %s", resp.StatusCode, body)
	}

	// Create/replace index template.
	dataStreamIndexTemplate := fmt.Sprintf(dataStreamIndexTemplate, indexPattern, ilmPolicyName)
	resp, err = esapi.IndicesPutIndexTemplateRequest{
		Name: indexTemplateName,
		Body: strings.NewReader(dataStreamIndexTemplate),
	}.Do(ctx, client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to create index template (%d): %s", resp.StatusCode, body)
	}
	return nil
}

const ilmPolicy = `{
  "policy": {
    "phases": {
      "hot": {
        "actions": {
          "rollover": {
            "max_age": "1h"
          }
        }
      },
      "delete": {
        "min_age": "1h",
        "actions": {
          "delete": {}
        }
      }
    }
  }
}`

const dataStreamIndexTemplate = `{
  "index_patterns": [%q],
  "priority": 1,
  "data_stream": {},
  "template": {
    "settings": {
      "index.lifecycle.name": %q
    },
    "mappings": {
      "properties": {
        "@timestamp": {"type": "date"},
        "event": {
          "properties": {
            "ingested": {"type": "date"}
          }
        },
        "observer": {
          "properties": {
            "id": {"type": "keyword"}
          }
        },
        "trace": {
          "properties": {
            "id": {"type": "keyword"}
          }
        }
      }
    }
  }
}`
