package transaction

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanTransform(t *testing.T) {
	var span *Span
	serviceName := "myService"
	service := m.Service{Name: &serviceName}

	path, path2 := "test/path", "second path"
	lineno := 56
	parent := 12
	tid := 1
	name := "myspan"
	spanType := "db query"
	start, startMicros := 9.8, 9800
	duration, durationMicros := 1.2, 1200
	err := "Colno and Lineno mandatory for sourcemapping."

	tests := []struct {
		Span   *Span
		Output common.MapStr
		Msg    string
	}{
		{
			Span:   span,
			Output: nil,
			Msg:    "Nil Span",
		},
		{
			Span:   &Span{},
			Output: nil,
			Msg:    "Empty Span",
		},
		{
			Span: &Span{
				Id:       &tid,
				Name:     &name,
				Type:     &spanType,
				Start:    &start,
				Duration: &duration,
				Stacktrace: []*m.StacktraceFrame{
					{AbsPath: &path},
					{AbsPath: &path2, Lineno: &lineno},
				},
				Context: common.MapStr{"key": "val"},
				Parent:  &parent,
			},
			Output: common.MapStr{
				"duration": common.MapStr{"us": &durationMicros},
				"id":       &tid,
				"name":     &name,
				"start":    common.MapStr{"us": &startMicros},
				"type":     &spanType,
				"parent":   &parent,
				"stacktrace": []common.MapStr{
					{
						"exclude_from_grouping": new(bool),
						"abs_path":              &path,
						"sourcemap": common.MapStr{
							"error":   &err,
							"updated": new(bool),
						},
					},
					{
						"exclude_from_grouping": new(bool),
						"abs_path":              &path2,
						"line": common.MapStr{
							"number": &lineno,
						},
						"sourcemap": common.MapStr{
							"error":   &err,
							"updated": new(bool),
						},
					},
				},
			},
			Msg: "Full Span",
		},
	}

	for idx, test := range tests {
		output := test.Span.Transform(&pr.Config{SmapMapper: &sourcemap.SmapMapper{}}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
