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

	"github.com/stretchr/testify/assert"

	"time"

	s "github.com/go-sourcemap/sourcemap"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func baseException() *Exception {
	return &Exception{ExMessage: "exception message"}
}

func (e *Exception) withCode(code interface{}) *Exception {
	e.ExCode = code
	return e
}

func (e *Exception) withType(etype string) *Exception {
	e.ExType = &etype
	return e
}

func (e *Exception) withFrames(frames []*m.StacktraceFrame) *Exception {
	e.ExStacktrace = m.Stacktrace(frames)
	return e
}

func baseLog() *Log {
	return &Log{LogMessage: "error log message"}
}

func (l *Log) withParamMsg(msg string) *Log {
	l.LogParamMessage = &msg
	return l
}

func (l *Log) withFrames(frames []*m.StacktraceFrame) *Log {
	l.LogStacktrace = m.Stacktrace(frames)
	return l
}

func TestEventTransform(t *testing.T) {
	id := "45678"
	culprit := "some trigger"

	errorType := "error type"
	codeFloat := 13.0
	module := "error module"
	exMsg := "exception message"
	handled := false
	attributes := common.MapStr{"k1": "val1"}
	exception := Exception{
		ExType:       &errorType,
		ExCode:       codeFloat,
		ExMessage:    exMsg,
		ExModule:     &module,
		ExHandled:    &handled,
		ExAttributes: attributes,
		ExStacktrace: []*m.StacktraceFrame{{Filename: "st file"}},
	}

	level := "level"
	loggerName := "logger"
	logMsg := "error log message"
	paramMsg := "param message"
	log := Log{
		LogLevel:        &level,
		LogMessage:      logMsg,
		LogParamMessage: &paramMsg,
		LoggerName:      &loggerName,
	}

	context := common.MapStr{"user": common.MapStr{"id": "888"}, "c1": "val"}

	baseExceptionHash := md5.New()
	io.WriteString(baseExceptionHash, baseException().ExMessage)
	// 706a38d554b47b8f82c6b542725c05dc
	baseExceptionGroupingKey := hex.EncodeToString(baseExceptionHash.Sum(nil))

	baseLogHash := md5.New()
	io.WriteString(baseLogHash, baseLog().LogMessage)
	baseLogGroupingKey := hex.EncodeToString(baseLogHash.Sum(nil))
	service := m.Service{Name: "myService"}

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event: Event{},
			Output: common.MapStr{
				"grouping_key": hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event",
		},
		{
			Event: Event{Log: baseLog()},
			Output: common.MapStr{
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseLogGroupingKey,
			},
			Msg: "Minimal Event wth log",
		},
		{
			Event: Event{Exception: baseException(), Log: baseLog()},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": "exception message"},
				"log":          common.MapStr{"message": "error log message"},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth log and exception",
		},
		{
			Event: Event{Exception: baseException()},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": "exception message"},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event with exception",
		},
		{
			Event: Event{Exception: baseException().withCode("13")},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": "exception message", "code": "13"},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event with exception and string code",
		},
		{
			Event: Event{Exception: baseException().withCode(13)},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": "exception message", "code": "13"},
				"grouping_key": baseExceptionGroupingKey,
			},
			Msg: "Minimal Event wth exception and int code",
		},
		{
			Event: Event{Exception: baseException().withCode(13.0)},
			Output: common.MapStr{
				"exception":    common.MapStr{"message": "exception message", "code": "13"},
				"grouping_key": baseExceptionGroupingKey,
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
				Transaction: &Transaction{"945254c5-67a5-417e-8a4e-aa29efcbfb79"},
			},
			Output: common.MapStr{
				"id":      "45678",
				"culprit": "some trigger",
				"exception": common.MapStr{
					"stacktrace": []common.MapStr{{
						"filename":              "st file",
						"line":                  common.MapStr{"number": 0},
						"exclude_from_grouping": false,
						"sourcemap": common.MapStr{
							"error":   "Colno mandatory for sourcemapping.",
							"updated": false,
						},
					}},
					"code":       "13",
					"message":    "exception message",
					"module":     "error module",
					"attributes": common.MapStr{"k1": "val1"},
					"type":       "error type",
					"handled":    false,
				},
				"log": common.MapStr{
					"message":       "error log message",
					"param_message": "param message",
					"logger_name":   "logger",
					"level":         "level",
				},
				"grouping_key": "d47ca09e1cfd512804f5d55cecd34262",
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
	c := "foo"
	fct := "fct"
	truthy := true
	st := m.Stacktrace{
		&m.StacktraceFrame{Filename: "a", Function: &fct, Sourcemap: m.Sourcemap{}},
	}
	stUpdate := m.Stacktrace{
		&m.StacktraceFrame{Filename: "a", Function: &fct, Sourcemap: m.Sourcemap{}},
		&m.StacktraceFrame{Filename: "a", LibraryFrame: &truthy, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: "f", Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
		&m.StacktraceFrame{Filename: "bar", Function: &fct, Sourcemap: m.Sourcemap{Updated: &truthy}},
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
			event:   Event{Culprit: &c, Log: &Log{LogStacktrace: st}},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "foo",
			msg:     "Log.StacktraceFrame has no updated frame",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					LogStacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  "f",
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
				Exception: &Exception{ExStacktrace: stUpdate},
			},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "f in fct",
			msg:     "Adapt culprit to first valid Exception.StacktraceFrame information.",
		},
		{
			event: Event{
				Culprit:   &c,
				Log:       &Log{LogStacktrace: st},
				Exception: &Exception{ExStacktrace: stUpdate},
			},
			config:  pr.Config{SmapMapper: &mapper},
			culprit: "f in fct",
			msg:     "Log and Exception StacktraceFrame given, only one changes culprit.",
		},
		{
			event: Event{
				Culprit: &c,
				Log: &Log{
					LogStacktrace: m.Stacktrace{
						&m.StacktraceFrame{
							Filename:  "a",
							Function:  &fct,
							Sourcemap: m.Sourcemap{Updated: &truthy},
						},
					},
				},
				Exception: &Exception{ExStacktrace: stUpdate},
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
	st1 := m.Stacktrace{
		&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 123, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "webpack", Lineno: 77, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "~/tmp", Lineno: 45, ExcludeFromGrouping: true},
	}
	st2 := m.Stacktrace{
		&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 123, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "webpack", Lineno: 77, ExcludeFromGrouping: false},
		&m.StacktraceFrame{Filename: "~/tmp", Lineno: 45, ExcludeFromGrouping: false},
	}
	e1 := Event{Exception: &Exception{ExMessage: "base exception", ExStacktrace: st1}}
	e2 := Event{Exception: &Exception{ExMessage: "base exception", ExStacktrace: st2}}
	key1 := e1.calcGroupingKey()
	key2 := e2.calcGroupingKey()
	assert.NotEqual(t, key1, key2)
}

func TestFallbackGroupingKey(t *testing.T) {
	lineno := 12
	filename := "file"

	groupingKey := hex.EncodeToString(md5With(filename, string(lineno)))

	e := Event{Exception: baseException().withFrames([]*m.StacktraceFrame{{Lineno: lineno, Filename: filename}})}
	assert.Equal(t, groupingKey, e.calcGroupingKey())

	e = Event{Exception: baseException(), Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: lineno, Filename: filename}})}
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
			{Lineno: lineno, Module: &module, Filename: filename, Function: &function},
		}),
	}
	assert.Equal(t, groupingKey, e.calcGroupingKey())
}

func TestGroupableEvents(t *testing.T) {
	value := "value"
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
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 10}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 57}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 10}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 57}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 0}}),
			},
			e2:     Event{},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: value}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: "name"}}),
			},
			e2: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: "name"}}),
			},
			result: false,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: "name"}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Module: &value, Filename: "nameEx"}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Filename: "name"}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Filename: "name"}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Lineno: 10}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Lineno: 10}}),
			},
			result: true,
		},
		{
			e1: Event{
				Log: baseLog().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 10}}),
			},
			e2: Event{
				Exception: baseException().withFrames([]*m.StacktraceFrame{{Function: &value, Lineno: 57}}),
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
	c1 := 18
	event := Event{Exception: &Exception{
		ExMessage: "exception message",
		ExStacktrace: m.Stacktrace{
			&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 1, Colno: &c1},
		},
	}}
	trNoSmap := event.Transform(&pr.Config{SmapMapper: nil}, m.Service{})

	event2 := Event{Exception: &Exception{
		ExMessage: "exception message",
		ExStacktrace: m.Stacktrace{
			&m.StacktraceFrame{Filename: "/a/b/c", Lineno: 1, Colno: &c1},
		},
	}}
	mapper := sourcemap.SmapMapper{Accessor: &fakeAcc{}}
	trWithSmap := event2.Transform(&pr.Config{SmapMapper: &mapper}, m.Service{})

	assert.Equal(t, 1, event.Exception.ExStacktrace[0].Lineno)
	assert.Equal(t, 5, event2.Exception.ExStacktrace[0].Lineno)

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
