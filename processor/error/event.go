package error

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        *string       `json:"id"`
	Culprit   *string       `json:"culprit"`
	Context   common.MapStr `json:"context"`
	Exception *Exception    `json:"exception"`
	Log       *Log          `json:"log"`
	Timestamp string        `json:"timestamp"`

	enhancer            utility.MapStrEnhancer
	data                common.MapStr
	TransformStacktrace m.TransformStacktrace
}

type Exception struct {
	Code             interface{}        `json:"code"`
	Message          string             `json:"message"`
	Module           *string            `json:"module"`
	Attributes       interface{}        `json:"attributes"`
	StacktraceFrames m.StacktraceFrames `json:"stacktrace"`
	Type             *string            `json:"type"`
	Uncaught         *bool              `json:"uncaught"`
}

type Log struct {
	Level            *string            `json:"level"`
	Message          string             `json:"message"`
	ParamMessage     *string            `json:"param_message"`
	LoggerName       *string            `json:"logger_name"`
	StacktraceFrames m.StacktraceFrames `json:"stacktrace"`
}

func (e *Event) DocType() string {
	return "error"
}

func (e *Event) Mappings(pa *Payload) (string, []m.DocMapping) {
	return e.Timestamp,
		[]m.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": e.DocType()}
			}},
			{Key: e.DocType(), Apply: e.Transform},
			{Key: "context", Apply: func() common.MapStr { return e.Context }},
			{Key: "context.app", Apply: pa.App.Transform},
			{Key: "context.system", Apply: pa.System.Transform},
		}
}

func (e *Event) Transform() common.MapStr {
	e.enhancer = utility.MapStrEnhancer{}
	e.data = common.MapStr{}

	e.add("id", e.Id)
	e.add("culprit", e.Culprit)

	e.addException()
	e.addLog()
	e.addGroupingKey()

	return e.data
}

func (e *Event) addException() {
	if e.Exception == nil {
		return
	}
	ex := common.MapStr{}
	e.enhancer.Add(ex, "message", e.Exception.Message)
	e.enhancer.Add(ex, "module", e.Exception.Module)
	e.enhancer.Add(ex, "attributes", e.Exception.Attributes)
	e.enhancer.Add(ex, "type", e.Exception.Type)
	e.enhancer.Add(ex, "uncaught", e.Exception.Uncaught)

	switch e.Exception.Code.(type) {
	case int:
		e.enhancer.Add(ex, "code", strconv.Itoa(e.Exception.Code.(int)))
	case float64:
		e.enhancer.Add(ex, "code", fmt.Sprintf("%.0f", e.Exception.Code))
	case string:
		e.enhancer.Add(ex, "code", e.Exception.Code.(string))
	}

	e.addStacktrace(ex, e.Exception.StacktraceFrames)

	e.add("exception", ex)
}

func (e *Event) addLog() {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	e.enhancer.Add(log, "message", e.Log.Message)
	e.enhancer.Add(log, "param_message", e.Log.ParamMessage)
	e.enhancer.Add(log, "logger_name", e.Log.LoggerName)
	e.enhancer.Add(log, "level", e.Log.Level)
	e.addStacktrace(log, e.Log.StacktraceFrames)

	e.add("log", log)
}

func (e *Event) addStacktrace(m common.MapStr, frames m.StacktraceFrames) {
	stacktrace := e.transformStacktrace(frames)
	if len(stacktrace) > 0 {
		e.enhancer.Add(m, "stacktrace", stacktrace)
	}
}

func (e *Event) transformStacktrace(frames m.StacktraceFrames) []common.MapStr {
	if e.TransformStacktrace == nil {
		e.TransformStacktrace = (*m.Stacktrace).Transform
	}
	st := m.Stacktrace{Frames: frames}
	return e.TransformStacktrace(&st)
}

func (e *Event) addGroupingKey() {
	e.add("grouping_key", e.calcGroupingKey())
}

func (e *Event) calcGroupingKey() string {
	hash := md5.New()

	add := func(s *string) bool {
		if s != nil {
			io.WriteString(hash, *s)
		}
		return s != nil
	}

	addEither := func(s *string, s2 string) {
		if ok := add(s); ok == false {
			add(&s2)
		}
	}

	var frames m.StacktraceFrames
	if e.Exception != nil {
		add(e.Exception.Type)
		frames = e.Exception.StacktraceFrames
	}
	if e.Log != nil {
		add(e.Log.ParamMessage)
		if frames == nil || len(frames) == 0 {
			frames = e.Log.StacktraceFrames
		}
	}

	for _, st := range frames {
		addEither(st.Module, st.Filename)
		addEither(st.Function, string(st.Lineno))
	}

	return hex.EncodeToString(hash.Sum(nil))
}

func (e *Event) add(key string, val interface{}) {
	e.enhancer.Add(e.data, key, val)
}
