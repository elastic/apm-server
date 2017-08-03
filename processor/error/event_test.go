package error

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

func TestEventTransform(t *testing.T) {
	id := "45678"
	culprit := "some trigger"
	ts := "2017-05-09T15:04:05.999999Z"

	errorType := "error type"
	codeFloat := 13.0
	module := "error module"
	exMsg := "exception message"
	uncaught := true
	attributes := common.MapStr{"k1": "val1"}
	exception := Exception{
		Type:             &errorType,
		Code:             codeFloat,
		Message:          &exMsg,
		Module:           &module,
		Uncaught:         &uncaught,
		Attributes:       attributes,
		StacktraceFrames: []m.StacktraceFrame{{Filename: "st file"}},
	}

	level := "level"
	loggerName := "logger"
	logMsg := "error log message"
	paramMsg := "param message"
	log := Log{
		Level:        &level,
		Message:      &logMsg,
		ParamMessage: &paramMsg,
		LoggerName:   &loggerName,
	}

	context := common.MapStr{"user": common.MapStr{"id": "888"}, "c1": "val"}

	emptyOut := common.MapStr{
		"checksum": hex.EncodeToString(md5.New().Sum(nil)),
	}

	tests := []struct {
		Event  Event
		Output common.MapStr
		Msg    string
	}{
		{
			Event:  Event{},
			Output: emptyOut,
			Msg:    "Minimal Event, default stacktrace transformation fn",
		},
		{
			Event: Event{Exception: Exception{Code: "13"}},
			Output: common.MapStr{
				"checksum": hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event, default stacktrace transformation fn",
		},
		{
			Event: Event{Log: Log{Message: &logMsg}},
			Output: common.MapStr{
				"log":      common.MapStr{"message": "error log message"},
				"checksum": hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event wth log, default stacktrace transformation fn",
		},
		{
			Event: Event{Exception: Exception{Message: &exMsg, Code: "13"}},
			Output: common.MapStr{
				"exception": common.MapStr{"message": "exception message", "code": "13"},
				"checksum":  hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event wth exception, string code, default stacktrace transformation fn",
		},
		{
			Event: Event{Exception: Exception{Message: &exMsg, Code: 13}},
			Output: common.MapStr{
				"exception": common.MapStr{"message": "exception message", "code": "13"},
				"checksum":  hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event wth exception, int code, default stacktrace transformation fn",
		},
		{
			Event: Event{Exception: Exception{Message: &exMsg, Code: 13.0}},
			Output: common.MapStr{
				"exception": common.MapStr{"message": "exception message", "code": "13"},
				"checksum":  hex.EncodeToString(md5.New().Sum(nil)),
			},
			Msg: "Minimal Event wth exception, float code, default stacktrace transformation fn",
		},
		{
			Event: Event{
				Id:        &id,
				Timestamp: ts,
				Culprit:   &culprit,
				Context:   context,
				Exception: exception,
				Log:       log,
			},
			Output: common.MapStr{
				"id":      "45678",
				"culprit": "some trigger",
				"exception": common.MapStr{
					"stacktrace": []common.MapStr{
						{"filename": "st file", "line": common.MapStr{"number": 0}},
					},
					"code":       "13",
					"message":    "exception message",
					"module":     "error module",
					"attributes": common.MapStr{"k1": "val1"},
					"type":       "error type",
					"uncaught":   true,
				},
				"log": common.MapStr{
					"message":       "error log message",
					"param_message": "param message",
					"logger_name":   "logger",
					"level":         "level",
				},
				"checksum": "8f9f94692ac3850d564a54d4ed69b0ad",
			},
			Msg: "Full Event with frames",
		},
	}

	for idx, test := range tests {
		output := test.Event.Transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestEmptyChecksum(t *testing.T) {
	emptyCheckSum := hex.EncodeToString(md5.New().Sum(nil))
	e := Event{}
	assert.Equal(t, emptyCheckSum, e.calcChecksum())
}

func TestExplicitChecksum(t *testing.T) {
	attr := "hello world"
	diffAttr := "huhu"

	checkSum := hex.EncodeToString(md5With(attr))

	e1 := Event{Log: Log{ParamMessage: &attr}}
	e2 := Event{Exception: Exception{Type: &attr}}
	e3 := Event{Log: Log{StacktraceFrames: []m.StacktraceFrame{{Function: &attr}}}}
	e4 := Event{Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Function: &attr}}}}
	e5 := Event{
		Log:       Log{StacktraceFrames: []m.StacktraceFrame{{Function: &diffAttr}}},
		Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Function: &attr}}},
	}

	for idx, e := range []Event{e1, e2, e3, e4, e5} {
		assert.Equal(t, checkSum, e.calcChecksum(), "checksum mismatch", idx)
	}
}

func TestFallbackChecksum(t *testing.T) {
	lineno := 12
	filename := "file"

	checkSum := hex.EncodeToString(md5With(filename, string(lineno)))

	e := Event{Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Lineno: lineno, Filename: filename}}}}
	assert.Equal(t, checkSum, e.calcChecksum())

	e = Event{Exception: Exception{}, Log: Log{StacktraceFrames: []m.StacktraceFrame{{Lineno: lineno, Filename: filename}}}}
	assert.Equal(t, checkSum, e.calcChecksum())
}

func TestNoFallbackChecksum(t *testing.T) {
	lineno := 1
	function := "function"
	filename := "file"
	module := "module"

	checkSum := hex.EncodeToString(md5With(module, function))

	e := Event{
		Exception: Exception{
			StacktraceFrames: []m.StacktraceFrame{
				{Lineno: lineno, Module: &module, Filename: filename, Function: &function},
			},
		},
	}
	assert.Equal(t, checkSum, e.calcChecksum())
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
				Log: Log{ParamMessage: &value},
			},
			e2: Event{
				Log: Log{ParamMessage: &value},
			},
			result: true,
		},
		{
			e1: Event{
				Exception: Exception{Type: &value},
			},
			e2: Event{
				Exception: Exception{Type: &value},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{ParamMessage: &value}, Exception: Exception{Type: &value},
			},
			e2: Event{
				Log: Log{ParamMessage: &value}, Exception: Exception{Type: &value},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{ParamMessage: &value}, Exception: Exception{Type: &value},
			},
			e2: Event{
				Log: Log{ParamMessage: &value},
			},
			result: false,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Function: &value, Lineno: 10}}},
			},
			e2: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Function: &value, Lineno: 57}}},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Lineno: 10}}},
			},
			e2: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Lineno: 57}}},
			},
			result: false,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Lineno: 0}}},
			},
			e2:     Event{},
			result: false,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Module: &value}}},
			},
			e2: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Filename: value}}},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Filename: "name"}}},
			},
			e2: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Module: &value, Filename: "name"}}},
			},
			result: false,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Module: &value, Filename: "name"}}},
			},
			e2: Event{
				Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Module: &value, Filename: "nameEx"}}},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Filename: "name"}}},
			},
			e2: Event{
				Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Filename: "name"}}},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Lineno: 10}}},
			},
			e2: Event{
				Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Lineno: 10}}},
			},
			result: true,
		},
		{
			e1: Event{
				Log: Log{StacktraceFrames: []m.StacktraceFrame{{Function: &value, Lineno: 10}}},
			},
			e2: Event{
				Exception: Exception{StacktraceFrames: []m.StacktraceFrame{{Function: &value, Lineno: 57}}},
			},
			result: true,
		},
	}

	for _, test := range tests {
		sameGroup := test.e1.calcChecksum() == test.e2.calcChecksum()
		assert.Equal(t, test.result, sameGroup,
			"checksum mismatch", test.e1.Id, test.e2.Id)
	}
}

func md5With(args ...string) []byte {
	md5 := md5.New()
	for _, arg := range args {
		md5.Write([]byte(arg))
	}
	return md5.Sum(nil)
}
