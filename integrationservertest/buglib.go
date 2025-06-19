package integrationservertest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// hasNestedField checks if a nested field (dot-separated path) exists in the properties map.
func hasNestedField(mappings types.TypeMapping, fieldPath string) bool {
	parts := strings.Split(fieldPath, ".")
	current := mappings.Properties
	// fmt.Println("parts", parts)
	// fmt.Println("curre", current)
	for i, part := range parts {
		// Check if the part exists in the current properties map
		val, exists := current[part]
		// fmt.Println("val", val, "parts", i == len(parts)-1)
		if !exists {
			return false
		}
		// If this is the last part, the field exists
		if i == len(parts)-1 {
			return true
		}
		// Try to descend into sub-properties (for nested/object fields)
		fieldMap, ok := val.(*types.ObjectProperty)
		if !ok {
			return false
		}
		// fmt.Println("fieldMap", fieldMap)
		subProps := fieldMap.Properties
		current = subProps
	}
	return false
}

type checkMappingStep struct {
	datastreamname string
	indexName      *regexp.Regexp
	checkFn        func(mappings types.TypeMapping) error
}

func (c checkMappingStep) Step(t *testing.T, ctx context.Context, e *testStepEnv) {
	t.Logf("------ check data stream mappings in %s ------", e.currentVersion())
	t.Logf("checking data stream mapping on %s", c.datastreamname)
	err := c.checkDataStreamMappings(t, ctx, e.esc.TypedClient())
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	t.Log("success")
}

func (c checkMappingStep) checkDataStreamMappings(t *testing.T, ctx context.Context, es *elasticsearch.TypedClient) error {
	dataStreamName := c.datastreamname

	info, err := es.Indices.GetDataStream().Name(dataStreamName).Do(ctx)
	if err != nil {
		return fmt.Errorf("Error getting data stream info: %s", err)
	}
	is := []string{}
	for _, v := range info.DataStreams[0].Indices {
		is = append(is, v.IndexName)
	}
	t.Logf("indices: %s", is)

	var backingIndex string
	for _, v := range info.DataStreams[0].Indices {
		if c.indexName.Match([]byte(v.IndexName)) {
			backingIndex = v.IndexName
		}
	}
	if backingIndex == "" {
		// collect available indices for easier troubleshooting
		indices := []string{}
		for _, v := range info.DataStreams[0].Indices {
			indices = append(indices, v.IndexName)
		}
		return errors.New(fmt.Sprintf("no index matches the filter regexp; filter=%s indices=%s", c.indexName, indices))
	}

	mappingRes, err := es.Indices.GetMapping().Index(backingIndex).Do(ctx)
	if err != nil {
		return fmt.Errorf("Error getting mapping: %w", err)
	}

	indexMappingRecord, ok := mappingRes[backingIndex]
	if !ok {
		return errors.New(fmt.Sprintf("Backing index %s not found in mapping", backingIndex))
	}

	if err := c.checkFn(indexMappingRecord.Mappings); err != nil {
		return fmt.Errorf("mapping check failed: %w", err)
	}

	return nil
}

type checkFieldExistsInDocsStep struct {
	dataStreamName string
	fieldName      string
	// checkFn can be used to customize the check this step performs on the returned
	// doc count. By default it checks if the vlaue is greater than 0.
	checkFn func(i int64) bool
}

func (s checkFieldExistsInDocsStep) Step(t *testing.T, ctx context.Context, e *testStepEnv) {
	t.Logf("------ check data stream docs with field %s in %s ------", s.fieldName, e.currentVersion())
	count, err := getDocCountWithField(ctx, e.esc.TypedClient(), s.dataStreamName, s.fieldName)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	t.Logf("documents in data stream %s with '%s': %d\n", s.dataStreamName, s.fieldName, count)

	if s.checkFn == nil {
		s.checkFn = func(i int64) bool { return i > 0 }
	}

	if !s.checkFn(count) {
		t.Log("checkFieldExistsInDocsStep: check function failed")
		t.Fail()
		return
	}
	t.Log("check successful")
}

func getDocCountWithField(ctx context.Context, c *elasticsearch.TypedClient, dataStreamName, fieldName string) (int64, error) {
	req := &count.Request{
		Query: &types.Query{
			Exists: &types.ExistsQuery{
				Field: fieldName,
			},
		},
	}
	resp, err := c.Core.Count().
		Index(dataStreamName).
		Request(req).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot retrieve doc count: %w", err)
	}
	return resp.Count, nil
}
