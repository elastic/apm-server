package error

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        *string
	Culprit   *string
	Context   common.MapStr
	Exception *Exception
	Log       *Log
	Timestamp time.Time

	enhancer utility.MapStrEnhancer
	data     common.MapStr
}

type Exception struct {
	Code       interface{}
	Message    string
	Module     *string
	Attributes interface{}
	Stacktrace m.Stacktrace `mapstructure:"stacktrace"`
	Type       *string
	Uncaught   *bool
}

type Log struct {
	Level        *string
	Message      string
	ParamMessage *string      `mapstructure:"param_message"`
	LoggerName   *string      `mapstructure:"logger_name"`
	Stacktrace   m.Stacktrace `mapstructure:"stacktrace"`
}

func (e *Event) DocType() string {
	return "error"
}

func (e *Event) Mappings(pa *payload, smapAccessor utility.SmapAccessor) (time.Time, []utility.DocMapping) {
	return e.Timestamp,
		[]utility.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": e.DocType()}
			}},
			{Key: e.DocType(), Apply: func() common.MapStr { return e.Transform(pa.Service, smapAccessor) }},
			{Key: "context", Apply: func() common.MapStr { return e.Context }},
			{Key: "context.service", Apply: pa.Service.Transform},
			{Key: "context.system", Apply: pa.System.Transform},
		}
}

func (e *Event) Transform(service m.Service, smapAccessor utility.SmapAccessor) common.MapStr {
	e.enhancer = utility.MapStrEnhancer{}
	e.data = common.MapStr{}

	e.add("id", e.Id)
	e.add("culprit", e.Culprit)

	e.addException(service, smapAccessor)
	e.addLog(service, smapAccessor)
	e.addGroupingKey()

	return e.data
}

func (e *Event) addException(service m.Service, smapAccessor utility.SmapAccessor) {
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

	st := e.Exception.Stacktrace.Transform(service, smapAccessor)
	if len(st) > 0 {
		e.enhancer.Add(ex, "stacktrace", st)
	}

	e.add("exception", ex)
}

func (e *Event) addLog(service m.Service, smapAccessor utility.SmapAccessor) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	e.enhancer.Add(log, "message", e.Log.Message)
	e.enhancer.Add(log, "param_message", e.Log.ParamMessage)
	e.enhancer.Add(log, "logger_name", e.Log.LoggerName)
	e.enhancer.Add(log, "level", e.Log.Level)
	st := e.Log.Stacktrace.Transform(service, smapAccessor)
	if len(st) > 0 {
		e.enhancer.Add(log, "stacktrace", st)
	}

	e.add("log", log)
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

	var st m.Stacktrace
	if e.Exception != nil {
		add(e.Exception.Type)
		st = e.Exception.Stacktrace
	}
	if e.Log != nil {
		add(e.Log.ParamMessage)
		if st == nil || len(st) == 0 {
			st = e.Log.Stacktrace
		}
	}

	for _, fr := range st {
		addEither(fr.Module, fr.Filename)
		addEither(fr.Function, string(fr.Lineno))
	}

	return hex.EncodeToString(hash.Sum(nil))
}

func (e *Event) add(key string, val interface{}) {
	e.enhancer.Add(e.data, key, val)
}
