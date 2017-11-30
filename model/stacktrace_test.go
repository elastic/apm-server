package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceTransform(t *testing.T) {
	service := Service{Name: "myService"}

	tests := []struct {
		Stacktrace Stacktrace
		Output     []common.MapStr
		Msg        string
	}{
		{
			Stacktrace: Stacktrace{StacktraceFrame{}},
			Output: []common.MapStr{
				{"filename": "", "line": common.MapStr{"number": 0}},
			},
			Msg: "Stacktrace with empty Frame",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.Transform(service, nil)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
