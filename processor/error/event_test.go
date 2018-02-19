package error

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	s "github.com/go-sourcemap/sourcemap"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func baseException() *Exception {
	msg := "exception message"
	return &Exception{Message: &msg}
}

func (e *Exception) withCode(code interface{}) *Exception {
	e.Code = code
	return e
}

func (e *Exception) withType(etype string) *Exception {
	e.Type = &etype
	return e
}

func (e *Exception) withFrames(frames []*m.StacktraceFrame) *Exception {
	e.Stacktrace = m.Stacktrace(frames)
	return e
}

func baseLog() *Log {
	msg := "error log message"
	return &Log{Message: &msg}
}

func (l *Log) withParamMsg(msg string) *Log {
	l.ParamMessage = &msg
	return l
}

func (l *Log) withFrames(frames []*m.StacktraceFrame) *Log {
	l.Stacktrace = m.Stacktrace(frames)
	return l
}

func TestEventTransform(t *testing.T) {
	serviceName := "myService"

	id := "45678"
	culprit := "some trigger"
	errorType := "error type"
	codeFloat, codeStr := 13.0, "13"
	module := "error module"
	exMsg := "exception message"
	handled := false
	attributes := common.MapStr{"k1": "val1"}
	filename := "st file"
	smapErr := "Colno and Lineno mandatory for sourcemapping."
	l0 := 0

	exception := Exception{
		Type:       &errorType,
		Code:       &codeFloat,
		Message:    &exMsg,
		Module:     &module,
		Handled:    &handled,
		Attributes: attributes,
		Stacktrace: []*m.StacktraceFrame{{Filename: &filename, Lineno: &l0}},
	}

	level := "level"
	loggerName := "logger"
	logMsg := "error log message"
	paramMsg := "param message"
	gKey := "d47ca09e1cfd512804f5d55cecd34262"
	log := Log{
		Level:        &level,
		Message:      &logMsg,
		ParamMessage: &paramMsg,
		LoggerName:   &loggerName,
	}

	context := common.MapStr{"user": common.MapStr{"id": "888"}, "c1": "val"}

	baseExceptionHash := md5.New()
	io.WriteString(baseExceptionHash, *baseException().Message)
	// 706a38d554b47b8f82c6b542725c05dc
	baseExceptionGroupingKey := hex.EncodeToString(baseExceptionHash.Sum(nil))

	baseLogHash := md5.New()
	io.WriteString(baseLogHash, *baseLog().Message)
	baseLogGroupingKey := hex.EncodeToString(baseLogHash.Sum(nil))
	service := m.Service{Name: &serviceName}
	nulKey := hex.EncodeToString(md5.New().Sum(nil))

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event:  Event{},
			Output: common.MapStr{"grouping_key": &nulKey},
			Msg:    "Minimal Event",
		},
		{
			Event: Event{Log: baseLog()},
			Output: common.MapStr{
				"log":          common.MapStr{"message": &logMsg},
				"grouping_key": &baseLogGroupingKey,
			},
			Msg: "Minimal Event wth log",
		},
		{
			Event: Event{Exception: baseException(), Log: baseLog()},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": &exMsg},
				"log":          common.MapStr{"message": &logMsg},
				"grouping_key": &baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth log and exception",
		},
		{
			Event: Event{Exception: baseException()},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": &exMsg},
				"grouping_key": &baseExceptionGroupingKey,
			},
			Msg: "Minimal Event with exception",
		},
		{
			Event: Event{Exception: baseException().withCode("13")},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": &exMsg, "code": &codeStr},
				"grouping_key": &baseExceptionGroupingKey,
			},
			Msg: "Minimal Event with exception and string code",
		},
		{
			Event: Event{Exception: baseException().withCode(13)},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": &exMsg, "code": &codeStr},
				"grouping_key": &baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth exception and int code",
		},
		{
			Event: Event{Exception: baseException().withCode(13.0)},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": &exMsg, "code": &codeStr},
				"grouping_key": &baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth exception and float code",
		},
		{
			Event: Event{
				Id:          &id,
				Timestamp:   time.Now(),
				Culprit:     &culprit,
				Context:     context,
				Exception:   &exception,
				Log:         &log,
				Transaction: &struct{ Id string }{Id: "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
			},
			Output: common.MapStr{
				"id":      &id,
				"culprit": &culprit,
				"exception": common.MapStr{
					"stacktrace": []common.MapStr{{
						"line":                  common.MapStr{"number": &l0},
						"filename":              &filename,
						"exclude_from_grouping": new(bool),
						"sourcemap": common.MapStr{
							"error":   &smapErr,
							"updated": new(bool),
						},
					}},
					"message":    &exMsg,
					"module":     &module,
					"attributes": common.MapStr{"k1": "val1"},
					"type":       &errorType,
					"handled":    new(bool),
				},
				"log": common.MapStr{
					"message":       &logMsg,
					"param_message": &paramMsg,
					"logger_name":   &loggerName,
					"level":         &level,
				},
				"grouping_key": &gKey,
			},
			Msg: "Full Event with frames",
		},
	}

	for idx, test := range tests {
		output := test.Event.Transform(&pr.Config{SmapMapper: &sourcemap.SmapMapper{}}, service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestCulprit(t *testing.T) {
	fna, fnf, fnbar := "a", "f", "bar"
	c := "foo"
	fct := "fct"
	truthy := true
	st := m.Stacktrace{
		&m.StacktraceFrame{Filename: &fna, Function: &fct, Sourcemap: m.Sourcemap{}},
	}
	stUpdate := m.Stacktrace{
		&m.StacktraceFrame{Filename: &fna, Function: &fct, Sourcemap: m.Sourcemap{}},
		&m.StacktraceFrame{Filename: &fna, LibraryFrame: &truthy, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: &fnf, Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: &fnbar, Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
	}
	mapper := sourcemap.SmapMapper{}
	tests := []struct {
		event   Event
		config  pr.Config
		culprit string
		msg     string
	}{
		{
			event:   Event{Culprit: &c},
			config:  pr.Config{},
			culprit: "foo",
			msg:     "No Sourcemap in config",
		},
		{
			event:   Event{Culprit: &c},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "foo",
			msg:     "No Stacktrace Frame given.",
		},
		{
			event:   Event{Culprit: &c, Log: &Log{Stacktrace: st}},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "foo",
			msg:     "Log.StacktraceFrame has no updated frame",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  &fnf,
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
			},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "f",
			msg:     "Adapt culprit to first valid Log.StacktraceFrame information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "f in fct",
			msg:     "Adapt culprit to first valid Exception.StacktraceFrame information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Log:       &Log{Stacktrace: st},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "f in fct",
			msg:     "Log and Exception StacktraceFrame given, only one changes culprit.",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					Stacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  &fna,
							Function:  &fct,
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
				Exception: &Exception{Stacktrace: stUpdate},
			},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "a in fct",
			msg:     "Log Stacktrace is prioritized over Exception StacktraceFrame",
		},
	}
	for idx, test := range tests {
		test.event.updateCulprit(&test.config)
		assert.Equal(t, test.culprit, *test.event.Culprit,
			fmt.Sprintf("(%v) expected <%v>, received <%v>", idx, test.culprit, *test.event.Culprit))
	}
}

func TestEmptyGroupingKey(t *testing.T) {
	emptyGroupingKey := hex.EncodeToString(md5.New().Sum(nil))
	e := Event{}
	assert.Equal(t, emptyGroupingKey, e.calcGroupingKey())
}

func TestExplicitGroupingKey(t *testing.T) {
	attr := "hello world"
	diffAttr := "huhu"

	groupingKey := hex.EncodeToString(md5With(attr))

	e1 := Event{Log: baseLog().withParamMsg(attr)}
	e2 := Event{Exception: baseException().withType(attr)}
	e3 := Event{Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &attr}})}
	e4 := Event{Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &attr}})}
	e5 := Event{
		Log:       baseLog().withFrames([]*m.StacktraceFrame{{Function: &diffAttr}}),
		Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &attr}}),
	}

	for idx, e := range []Event{e1, e2, e3, e4, e5} {
		assert.Equal(t, groupingKey, e.calcGroupingKey(), "grouping_key mismatch", idx)
	}
}

func TestFramesUsableForGroupingKey(t *testing.T) {
	fna, fnwebpack, fntmp := "/a/b/c", "webpack", "~/tmp"
	l123, l77, l45 := 123, 77, 45
	truthy := true
	msg := "base exc"
	st1 := m.Stacktrace{
		&m.StacktraceFrame{Filename: &fna, Lineno: &l123, ExcludeFromGrouping: new(bool)},
		&m.StacktraceFrame{Filename: &fnwebpack, Lineno: &l77, ExcludeFromGrouping: new(bool)},
		&m.StacktraceFrame{Filename: &fntmp, Lineno: &l45, ExcludeFromGrouping: &truthy},
	}
	st2 := m.Stacktrace{
		&m.StacktraceFrame{Filename: &fna, Lineno: &l123, ExcludeFromGrouping: new(bool)},
		&m.StacktraceFrame{Filename: &fnwebpack, Lineno: &l77, ExcludeFromGrouping: new(bool)},
		&m.StacktraceFrame{Filename: &fntmp, Lineno: &l45, ExcludeFromGrouping: new(bool)},
	}
	e1 := Event{Exception: &Exception{Message: &msg, Stacktrace: st1}}
	e2 := Event{Exception: &Exception{Message: &msg, Stacktrace: st2}}
	key1 := e1.calcGroupingKey()
	key2 := e2.calcGroupingKey()
	assert.NotEqual(t, key1, key2)
}

func TestFallbackGroupingKey(t *testing.T) {
	lineno := 12
	filename := "file"

	groupingKey := hex.EncodeToString(md5With(filename, string(lineno)))

	e := Event{Exception: baseException().withFrames([]*m.StacktraceFrame{{Lineno: &lineno, Filename: &filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey())

	e = Event{Exception: baseException(), Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: &lineno, Filename: &filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey())
}

func TestNoFallbackGroupingKey(t *testing.T) {
	lineno := 1
	function := "function"
	filename := "file"
	module := "module"

	groupingKey := hex.EncodeToString(md5With(module, function))

	e := Event{
		Exception: baseException().withFrames([]*m.StacktraceFrame{
			{Lineno: &lineno, Module: &module, Filename: &filename, Function: &function},
		}),
	}
	assert.Equal(t, groupingKey, e.calcGroupingKey())
}

func TestGroupableEvents(t *testing.T) {
	value, fnName := "value", "name"
	ln0, ln10, ln57 := 0, 10, 57
	var tests = []struct {
		e1     Event
		e2     Event
		result bool
	}{
		{
			e1: Event{
				Log: baseLog().withParamMsg(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value),
			},
			result: true,
		},
		{
			e1: Event{
				Exception: baseException().withType(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withParamMsg(value), Exception: baseException().withType(value),
			},
			e2: Event{
				Log: baseLog().withParamMsg(value),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: &ln10}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: &ln57}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: &ln10}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: &ln57}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: &ln0}}),
			},
			e2:     Event{},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: &value}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: &fnName}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: &fnName}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: &fnName}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: &fnName}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: &fnName}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Filename: &fnName}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: &ln10}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Lineno: &ln10}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: &ln10}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: &ln57}}),
			},
			result: true,
		},
	}

	for idx, test := range tests {
		sameGroup := test.e1.calcGroupingKey() == test.e2.calcGroupingKey()
		assert.Equal(t, test.result, sameGroup,
			"grouping_key mismatch", idx)
	}
}

func md5With(args ...string) []byte {
	md5 := md5.New()
	for _, arg := range args {
		md5.Write([]byte(arg))
	}
	return md5.Sum(nil)
}

func TestSourcemapping(t *testing.T) {
	l1, c1 := 1, 18
	msg := "exception message"
	name := "/a/b/c"

	event := Event{Exception: &Exception{
		Message: &msg,
		Stacktrace: m.Stacktrace{
			&m.StacktraceFrame{Filename: &name, Lineno: &l1, Colno: &c1},
		},
	}}
	trNoSmap := event.Transform(&pr.Config{SmapMapper: nil}, m.Service{})

	event2 := Event{Exception: &Exception{
		Message: &msg,
		Stacktrace: m.Stacktrace{
			&m.StacktraceFrame{Filename: &name, Lineno: &l1, Colno: &c1},
		},
	}}
	mapper := sourcemap.SmapMapper{Accessor: &fakeAcc{}}
	trWithSmap := event2.Transform(&pr.Config{SmapMapper: &mapper}, m.Service{})

	assert.Equal(t, 1, *event.Exception.Stacktrace[0].Lineno)
	assert.Equal(t, 5, *event2.Exception.Stacktrace[0].Lineno)

	assert.NotEqual(t, trNoSmap["grouping_key"], trWithSmap["grouping_key"])
	fmt.Println(trNoSmap)
}

type fakeAcc struct{}

func (ac *fakeAcc) Fetch(smapId sourcemap.Id) (*s.Consumer, error) {
	file := "bundle.js.map"
	current, _ := os.Getwd()
	path := filepath.Join(current, "../../tests/data/valid/sourcemap/", file)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return s.Parse("", fileBytes)
}
func (a *fakeAcc) Remove(smapId sourcemap.Id) {}
