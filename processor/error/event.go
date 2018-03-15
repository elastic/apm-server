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

func (e *Event) decode(raw map[string]interface{}) error {
	df := utility.DataFetcher{}
	e.Id = df.StringPtr(raw, "id")
	e.Culprit = df.StringPtr(raw, "culprit")
	e.Context = df.MapStr(raw, "context")
	e.Timestamp = df.TimeRFC3339(raw, "timestamp")
	transactionId := df.StringPtr(raw, "id", "transaction")
	if transactionId != nil {
		e.Transaction = &Transaction{Id: *transactionId}
	}
	if df.Err != nil {
		return df.Err
	}

	ex := df.MapStr(raw, "exception")
	exMsg := df.StringPtr(ex, "message")
	if exMsg != nil {
		e.Exception = &Exception{
			Message:    *exMsg,
			Code:       df.Interface(ex, "code"),
			Module:     df.StringPtr(ex, "module"),
			Attributes: df.Interface(ex, "attributes"),
			Type:       df.StringPtr(ex, "type"),
			Handled:    df.BoolPtr(ex, "handled"),
			Stacktrace: m.Stacktrace{},
		}
		if err := e.Exception.Stacktrace.Decode(ex["stacktrace"]); err != nil {
			return err
		}
	}
	if df.Err != nil {
		return df.Err
	}

	log := df.MapStr(raw, "log")
	logMsg := df.StringPtr(log, "message")
	if logMsg != nil {
		e.Log = &Log{
			Message:      *logMsg,
			ParamMessage: df.StringPtr(log, "param_message"),
			Level:        df.StringPtr(log, "level"),
			LoggerName:   df.StringPtr(log, "logger_name"),
			Stacktrace:   m.Stacktrace{},
		}
		if err := e.Log.Stacktrace.Decode(log["stacktrace"]); err != nil {
			return err
		}
	}
	return df.Err
}

func (e *Event) Transform(config *pr.Config, service m.Service) common.MapStr {
	e.data = common.MapStr{}
	e.add("id", e.Id)

	e.addException(config, service)
	e.addLog(config, service)

	e.updateCulprit(config)
	e.add("culprit", e.Culprit)

	e.addGroupingKey()

	return e.data
}

func (e *Event) updateCulprit(config *pr.Config) {
	if config == nil || config.SmapMapper == nil {
		return
	}
	var fr *m.StacktraceFrame
	if e.Log != nil {
		fr = findSmappedNonLibraryFrame(e.Log.Stacktrace.Frames)
	}
	if fr == nil && e.Exception != nil {
		fr = findSmappedNonLibraryFrame(e.Exception.Stacktrace.Frames)
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

func (e *Event) addException(config *pr.Config, service m.Service) {
	if e.Exception == nil {
		return
	}
	ex := common.MapStr{}
	utility.Add(ex, "message", e.Exception.Message)
	utility.Add(ex, "module", e.Exception.Module)
	utility.Add(ex, "attributes", e.Exception.Attributes)
	utility.Add(ex, "type", e.Exception.Type)
	utility.Add(ex, "handled", e.Exception.Handled)

	switch e.Exception.Code.(type) {
	case int:
		utility.Add(ex, "code", strconv.Itoa(e.Exception.Code.(int)))
	case float64:
		utility.Add(ex, "code", fmt.Sprintf("%.0f", e.Exception.Code))
	case string:
		utility.Add(ex, "code", e.Exception.Code.(string))
	}

	st := e.Exception.Stacktrace.Transform(config, service)
	utility.Add(ex, "stacktrace", st)

	e.add("exception", ex)
}

func (e *Event) addLog(config *pr.Config, service m.Service) {
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
		if st.Frames == nil || len(st.Frames) == 0 {
			st = e.Log.Stacktrace
		}
	}

	if st.Frames != nil {
		for _, fr := range st.Frames {
			if fr.ExcludeFromGrouping {
				continue
			}
			k.addEither(fr.Module, fr.Filename)
			k.addEither(fr.Function, string(fr.Lineno))
		}
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
