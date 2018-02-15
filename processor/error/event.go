package error

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strconv"
	"time"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

const errorDocType = "error"

type Event struct {
	Timestamp   time.Time
	Id          *string
	Culprit     *string
	Context     common.MapStr
	Exception   *Exception
	Log         *Log
	Transaction *struct {
		Id string
	}

	data common.MapStr
}

type Exception struct {
	Message    *string
	Code       interface{}
	Module     *string
	Attributes interface{}
	Stacktrace m.Stacktrace `mapstructure:"stacktrace"`
	Type       *string
	Handled    *bool
}

type Log struct {
	Message      *string
	Level        *string
	ParamMessage *string      `mapstructure:"param_message"`
	LoggerName   *string      `mapstructure:"logger_name"`
	Stacktrace   m.Stacktrace `mapstructure:"stacktrace"`
}

func (e *Event) Mappings(config *pr.Config, pa *payload) (time.Time, []utility.DocMapping) {
	mapping := []utility.DocMapping{
		{Key: "processor", Apply: func() common.MapStr {
			return common.MapStr{"name": processorName, "event": errorDocType}
		}},
		{Key: errorDocType, Apply: func() common.MapStr { return e.Transform(config, pa.Service) }},
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
	if e == nil {
		return nil
	}
	e.data = common.MapStr{}
	utility.AddStrPtr(e.data, "id", e.Id)

	e.addException(config, service)
	e.addLog(config, service)

	e.updateCulprit(config)
	utility.AddStrPtr(e.data, "culprit", e.Culprit)

	e.addGroupingKey()

	return e.data
}

func (e *Event) updateCulprit(config *pr.Config) {
	if config == nil || config.SmapMapper == nil {
		return
	}
	var fr *m.StacktraceFrame
	if e.Log != nil {
		fr = findSmappedNonLibraryFrame(e.Log.Stacktrace)
	}
	if fr == nil && e.Exception != nil {
		fr = findSmappedNonLibraryFrame(e.Exception.Stacktrace)
	}
	if fr == nil {
		return
	}
	culprit := fmt.Sprintf("%v", *fr.Filename)
	if fr.Function != nil {
		culprit += fmt.Sprintf(" in %v", *fr.Function)
	}
	e.Culprit = &culprit
}

func findSmappedNonLibraryFrame(frames []*m.StacktraceFrame) *m.StacktraceFrame {
	for _, fr := range frames {
		if fr.IsSourcemapApplied() && !fr.IsLibraryFrame() {
			return fr
		}
	}
	return nil
}

func (e *Event) addException(config *pr.Config, service m.Service) {
	if e.Exception == nil {
		return
	}
	ex := common.MapStr{}
	utility.AddStrPtr(ex, "message", e.Exception.Message)
	utility.AddStrPtr(ex, "module", e.Exception.Module)
	utility.AddStrPtr(ex, "type", e.Exception.Type)
	utility.AddBoolPtr(ex, "handled", e.Exception.Handled)
	utility.AddInterface(ex, "attributes", e.Exception.Attributes)

	switch e.Exception.Code.(type) {
	case int:
		code := strconv.Itoa(e.Exception.Code.(int))
		utility.AddStrPtr(ex, "code", &code)
	case float64:
		code := fmt.Sprintf("%.0f", e.Exception.Code)
		utility.AddStrPtr(ex, "code", &code)
	case string:
		code := e.Exception.Code.(string)
		utility.AddStrPtr(ex, "code", &code)
	}

	st := e.Exception.Stacktrace.Transform(config, service)
	if len(st) > 0 {
		utility.AddCommonMapStrArray(ex, "stacktrace", st)
	}
	utility.AddCommonMapStr(e.data, "exception", ex)
}

func (e *Event) addLog(config *pr.Config, service m.Service) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	utility.AddStrPtr(log, "message", e.Log.Message)
	utility.AddStrPtr(log, "param_message", e.Log.ParamMessage)
	utility.AddStrPtr(log, "logger_name", e.Log.LoggerName)
	utility.AddStrPtr(log, "level", e.Log.Level)
	st := e.Log.Stacktrace.Transform(config, service)
	if len(st) > 0 {
		utility.AddCommonMapStrArray(log, "stacktrace", st)
	}
	utility.AddCommonMapStr(e.data, "log", log)
}

func (e *Event) addGroupingKey() {
	groupingKey := e.calcGroupingKey()
	utility.AddStrPtr(e.data, "grouping_key", &groupingKey)
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

func (k *groupingKey) addEither(s1 *string, s2 *string) {
	if ok := k.add(s1); !ok {
		k.add(s2)
	}
}

func (k *groupingKey) String() string {
	return hex.EncodeToString(k.hash.Sum(nil))
}

// calcGroupingKey computes a value for deduplicating errors - events with
// same grouping key can be collapsed together.
func (e *Event) calcGroupingKey() string {
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
		if fr.IsExcludedFromGrouping() {
			continue
		}
		k.addEither(fr.Module, fr.Filename)
		if !k.add(fr.Function) && fr.Lineno != nil {
			linenoStr := string(*fr.Lineno)
			k.add(&linenoStr)
		}
	}

	if k.empty {
		if e.Exception != nil {
			k.add(e.Exception.Message)
		} else if e.Log != nil {
			k.add(e.Log.Message)
		}
	}

	return k.String()
}
