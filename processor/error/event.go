package error

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"strconv"
	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Event struct {
	Id        *string
	Culprit   *string
	Context   common.MapStr
	Timestamp time.Time

	Exception   *Exception
	Log         *Log
	Transaction *Transaction

	data common.MapStr
}

type Transaction struct {
	Id string
}

type Exception struct {
	Message    string
	Module     *string
	Code       interface{}
	Attributes interface{}
	Stacktrace m.Stacktrace
	Type       *string
	Handled    *bool
}

type Log struct {
	Message      string
	Level        *string
	ParamMessage *string
	LoggerName   *string
	Stacktrace   m.Stacktrace
}

func DecodeEvent(input interface{}, err error) (*Event, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for error event")
	}
	decoder := utility.ManualDecoder{}
	e := Event{
		Id:        decoder.StringPtr(raw, "id"),
		Culprit:   decoder.StringPtr(raw, "culprit"),
		Context:   decoder.MapStr(raw, "context"),
		Timestamp: decoder.TimeRFC3339WithDefault(raw, "timestamp"),
	}
	transactionId := decoder.StringPtr(raw, "id", "transaction")
	if transactionId != nil {
		e.Transaction = &Transaction{Id: *transactionId}
	}

	var stacktr *m.Stacktrace
	err = decoder.Err
	ex := decoder.MapStr(raw, "exception")
	exMsg := decoder.StringPtr(ex, "message")
	if exMsg != nil {
		e.Exception = &Exception{
			Message:    *exMsg,
			Code:       decoder.Interface(ex, "code"),
			Module:     decoder.StringPtr(ex, "module"),
			Attributes: decoder.Interface(ex, "attributes"),
			Type:       decoder.StringPtr(ex, "type"),
			Handled:    decoder.BoolPtr(ex, "handled"),
			Stacktrace: m.Stacktrace{},
		}
		stacktr, err = m.DecodeStacktrace(ex["stacktrace"], err)
		if stacktr != nil {
			e.Exception.Stacktrace = *stacktr
		}
	}

	log := decoder.MapStr(raw, "log")
	logMsg := decoder.StringPtr(log, "message")
	if logMsg != nil {
		e.Log = &Log{
			Message:      *logMsg,
			ParamMessage: decoder.StringPtr(log, "param_message"),
			Level:        decoder.StringPtr(log, "level"),
			LoggerName:   decoder.StringPtr(log, "logger_name"),
			Stacktrace:   m.Stacktrace{},
		}
		stacktr, err = m.DecodeStacktrace(log["stacktrace"], err)
		if stacktr != nil {
			e.Log.Stacktrace = *stacktr
		}
	}
	return &e, err
}

func (e *Event) Transform(config config.Config, service m.Service) common.MapStr {
	e.data = common.MapStr{}
	e.add("id", e.Id)

	e.addException(config, service)
	e.addLog(config, service)

	e.updateCulprit(config)
	e.add("culprit", e.Culprit)

	e.addGroupingKey()

	return e.data
}

func (e *Event) updateCulprit(config config.Config) {
	if config.SmapMapper == nil {
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
	culprit := fmt.Sprintf("%v", fr.Filename)
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

func (e *Event) addException(config config.Config, service m.Service) {
	if e.Exception == nil {
		return
	}
	ex := common.MapStr{}
	utility.Add(ex, "message", e.Exception.Message)
	utility.Add(ex, "module", e.Exception.Module)
	utility.Add(ex, "attributes", e.Exception.Attributes)
	utility.Add(ex, "type", e.Exception.Type)
	utility.Add(ex, "handled", e.Exception.Handled)

	switch code := e.Exception.Code.(type) {
	case int:
		utility.Add(ex, "code", strconv.Itoa(code))
	case float64:
		utility.Add(ex, "code", fmt.Sprintf("%.0f", code))
	case string:
		utility.Add(ex, "code", code)
	case json.Number:
		utility.Add(ex, "code", code.String())
	}

	st := e.Exception.Stacktrace.Transform(config, service)
	utility.Add(ex, "stacktrace", st)

	e.add("exception", ex)
}

func (e *Event) addLog(config config.Config, service m.Service) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	utility.Add(log, "message", e.Log.Message)
	utility.Add(log, "param_message", e.Log.ParamMessage)
	utility.Add(log, "logger_name", e.Log.LoggerName)
	utility.Add(log, "level", e.Log.Level)
	st := e.Log.Stacktrace.Transform(config, service)
	utility.Add(log, "stacktrace", st)

	e.add("log", log)
}

func (e *Event) addGroupingKey() {
	e.add("grouping_key", e.calcGroupingKey())
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
		if fr.ExcludeFromGrouping {
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
	utility.Add(e.data, key, val)
}
