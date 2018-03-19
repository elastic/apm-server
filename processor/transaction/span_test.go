package transaction

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanDecode(t *testing.T) {
	id, parent := 1, 12
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	context := common.MapStr{"a": "b"}
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1,
	}}
	for _, test := range []struct {
		input interface{}
		err   error
		s     *Span
	}{
		{input: nil, err: nil, s: &Span{}},
		{input: "", err: errors.New("Invalid type for span"), s: &Span{}},
		{
			input: map[string]interface{}{},
			err:   errors.New("Mandatory field missing"),
			s:     &Span{},
		},
		{
			input: map[string]interface{}{
				"name": name, "id": &id, "type": spType,
				"start": start, "duration": duration,
				"context": context, "parent": &parent,
				"stacktrace": stacktrace,
			},
			err: nil,
			s: &Span{
				Id:       &id,
				Name:     name,
				Type:     spType,
				Start:    start,
				Duration: duration,
				Context:  context,
				Parent:   &parent,
				Stacktrace: m.Stacktrace{
					Frames: []*m.StacktraceFrame{
						&m.StacktraceFrame{Filename: "file", Lineno: 1},
					},
				},
			},
		},
	} {
		span := &Span{}
		out := span.decode(test.input)
		assert.Equal(t, test.s, span)
		assert.Equal(t, test.err, out)
	}

	var s *Span
	assert.Nil(t, s.decode("a"), nil)
}
func TestSpanTransform(t *testing.T) {
	path := "test/path"
	parent := 12
	tid := 1
	service := m.Service{Name: "myService"}

	tests := []struct {
		Span   Span
		Output common.MapStr
		Msg    string
	}{
		{
			Span: Span{},
			Output: common.MapStr{
				"type":     "",
				"start":    common.MapStr{"us": 0},
				"duration": common.MapStr{"us": 0},
				"name":     "",
			},
			Msg: "Span without a Stacktrace",
		},
		{
			Span: Span{
				Id:       &tid,
				Name:     "myspan",
				Type:     "myspantype",
				Start:    0.65,
				Duration: 1.20,
				Stacktrace: m.Stacktrace{
					Frames: []*m.StacktraceFrame{
						{AbsPath: &path},
					},
				},
				Context: common.MapStr{"key": "val"},
				Parent:  &parent,
			},
			Output: common.MapStr{
				"duration": common.MapStr{"us": 1200},
				"id":       1,
				"name":     "myspan",
				"start":    common.MapStr{"us": 650},
				"type":     "myspantype",
				"parent":   12,
				"stacktrace": []common.MapStr{{
					"exclude_from_grouping": false,
					"abs_path":              path,
					"filename":              "",
					"line":                  common.MapStr{"number": 0},
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					},
				}},
			},
			Msg: "Full Span",
		},
	}

	for idx, test := range tests {
		output := test.Span.Transform(&pr.Config{SmapMapper: &sourcemap.SmapMapper{}}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
