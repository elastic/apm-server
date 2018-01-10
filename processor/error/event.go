package error

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id          *string
	Culprit     *string
	Context     common.MapStr
	Exception   *Exception
	Log         *Log
	Timestamp   time.Time
	Transaction *struct {
		Id string
	}

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
	Handled    *bool
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

func (e *Event) Mappings(config *pr.Config, pa *payload) (time.Time, []utility.DocMapping) {
	mapping := []utility.DocMapping{
		{Key: "processor", Apply: func() common.MapStr {
			return common.MapStr{"name": processorName, "event": e.DocType()}
		}},
		{Key: e.DocType(), Apply: func() common.MapStr { return e.Transform(config, pa.Service) }},
		{Key: "context", Apply: func() common.MapStr { return e.Context }},
		{Key: "context.service", Apply: pa.Service.Transform},
		{Key: "context.system", Apply: pa.System.Transform},
		{Key: "context.process", Apply: pa.Process.Transform},
	}

	if e.Transaction != nil {
		mapping = append(mapping, utility.DocMapping{
			Key:   "transaction",
			Apply: func() common.MapStr { return common.MapStr{"id": e.Transaction.Id} },
		})
	}

	return e.Timestamp, mapping
}

func (e *Event) Transform(config *pr.Config, service m.Service) common.MapStr {
	e.enhancer = utility.MapStrEnhancer{}
	e.data = common.MapStr{}

	e.add("id", e.Id)
	e.add("culprit", e.Culprit)

	e.addException(config, service)
	e.addLog(config, service)
	e.addGroupingKey(config)

	return e.data
}

func (e *Event) addException(config *pr.Config, service m.Service) {
	if e.Exception == nil {
		return
	}
	ex := common.MapStr{}
	e.enhancer.Add(ex, "message", e.Exception.Message)
	e.enhancer.Add(ex, "module", e.Exception.Module)
	e.enhancer.Add(ex, "attributes", e.Exception.Attributes)
	e.enhancer.Add(ex, "type", e.Exception.Type)
	e.enhancer.Add(ex, "handled", e.Exception.Handled)

	switch e.Exception.Code.(type) {
	case int:
		e.enhancer.Add(ex, "code", strconv.Itoa(e.Exception.Code.(int)))
	case float64:
		e.enhancer.Add(ex, "code", fmt.Sprintf("%.0f", e.Exception.Code))
	case string:
		e.enhancer.Add(ex, "code", e.Exception.Code.(string))
	}

	st := e.Exception.Stacktrace.Transform(config, service)
	if len(st) > 0 {
		e.enhancer.Add(ex, "stacktrace", st)
	}

	e.add("exception", ex)
}

func (e *Event) addLog(config *pr.Config, service m.Service) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	e.enhancer.Add(log, "message", e.Log.Message)
	e.enhancer.Add(log, "param_message", e.Log.ParamMessage)
	e.enhancer.Add(log, "logger_name", e.Log.LoggerName)
	e.enhancer.Add(log, "level", e.Log.Level)
	st := e.Log.Stacktrace.Transform(config, service)
	if len(st) > 0 {
		e.enhancer.Add(log, "stacktrace", st)
	}

	e.add("log", log)
}

func (e *Event) addGroupingKey(config *pr.Config) {
	e.add("grouping_key", e.calcGroupingKey(config))
}

type groupingKey struct {
	hash  hash.Hash
	empty bool
}

func newGroupingKey() *groupingKey {
	return &groupingKey{
		hash:  md5.New(),
		empty: true,
	}
}

func (k *groupingKey) add(s *string) bool {
	if s == nil {
		return false
	}
	io.WriteString(k.hash, *s)
	k.empty = false
	return true
}

func (k *groupingKey) addEither(s1 *string, s2 string) {
	if ok := k.add(s1); !ok {
		k.add(&s2)
	}
}

func (k *groupingKey) String() string {
	return hex.EncodeToString(k.hash.Sum(nil))
}

// calcGroupingKey computes a value for deduplicating errors - events with
// same grouping key can be collapsed together.
func (e *Event) calcGroupingKey(config *pr.Config) string {
	var isFrontendEvent bool
	if config != nil {
		isFrontendEvent = config.Frontend
	}

	k := newGroupingKey()

	var st m.Stacktrace
	if e.Exception != nil {
		k.add(e.Exception.Type)
		st = e.Exception.Stacktrace
	}
	if e.Log != nil {
		k.add(e.Log.ParamMessage)
		if st == nil || len(st) == 0 {
			st = e.Log.Stacktrace
		}
	}

	for _, fr := range st {
		if isFrontendEvent && !use_for_grouping(fr) {
			continue
		}
		k.addEither(fr.Module, fr.Filename)
		k.addEither(fr.Function, string(fr.Lineno))
	}

	if k.empty {
		if e.Exception != nil {
			k.add(&e.Exception.Message)
		} else if e.Log != nil {
			k.add(&e.Log.Message)
		}
	}

	return k.String()
}

func (e *Event) add(key string, val interface{}) {
	e.enhancer.Add(e.data, key, val)
}

func use_for_grouping(fr *m.StacktraceFrame) bool {
	if fr.Filename == "" || strings.HasPrefix(fr.Filename, "/webpack") {
		return false
	}
	file := filepath.Base(fr.Filename)
	if strings.HasPrefix(fr.Filename, ("/" + file)) {
		return false
	}
	return true
}
