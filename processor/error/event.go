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
	Id          *string
	Culprit     *string
	Context     common.MapStr
	*Exception
	*Log
	*Transaction
	Timestamp   time.Time

	data common.MapStr
}

type Transaction struct {
	TransactionId string
}

type Exception struct {
	ExCode       interface{}
	ExMessage    string
	ExModule     *string
	ExAttributes interface{}
	ExStacktrace m.Stacktrace
	ExType       *string
	ExHandled    *bool
}

type Log struct {
	LogLevel        *string
	LogMessage      string
	LogParamMessage *string
	LoggerName      *string
	LogStacktrace   m.Stacktrace
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

// This updates the event in place
func (e *Event) contextTransform(pa *payload) common.MapStr {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	utility.InsertInMap(e.Context, "user", pa.User)
	return e.Context
}

func (e *Event) updateCulprit(config *pr.Config) {
	if config == nil || config.SmapMapper == nil {
		return
	}
	var fr *m.StacktraceFrame
	if e.Log != nil {
		fr = findSmappedNonLibraryFrame(e.Log.LogStacktrace)
	}
	if fr == nil && e.Exception != nil {
		fr = findSmappedNonLibraryFrame(e.Exception.ExStacktrace)
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
	utility.Add(ex, "message", e.Exception.ExMessage)
	utility.Add(ex, "module", e.Exception.ExModule)
	utility.Add(ex, "attributes", e.Exception.ExAttributes)
	utility.Add(ex, "type", e.Exception.ExType)
	utility.Add(ex, "handled", e.Exception.ExHandled)

	switch e.Exception.ExCode.(type) {
	case int:
		utility.Add(ex, "code", strconv.Itoa(e.Exception.ExCode.(int)))
	case float64:
		utility.Add(ex, "code", fmt.Sprintf("%.0f", e.Exception.ExCode))
	case string:
		utility.Add(ex, "code", e.Exception.ExCode.(string))
	}

	st := e.Exception.ExStacktrace.Transform(config, service)
	utility.Add(ex, "stacktrace", st)

	e.add("exception", ex)
}

func (e *Event) addLog(config *pr.Config, service m.Service) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	utility.Add(log, "message", e.Log.LogMessage)
	utility.Add(log, "param_message", e.Log.LogParamMessage)
	utility.Add(log, "logger_name", e.Log.LoggerName)
	utility.Add(log, "level", e.Log.LogLevel)
	st := e.Log.LogStacktrace.Transform(config, service)
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
		k.add(e.Exception.ExType)
		st = e.Exception.ExStacktrace
	}
	if e.Log != nil {
		k.add(e.Log.LogParamMessage)
		if st == nil || len(st) == 0 {
			st = e.Log.LogStacktrace
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
			k.add(&e.Exception.ExMessage)
		} else if e.Log != nil {
			k.add(&e.Log.LogMessage)
		}
	}

	return k.String()
}

func (e *Event) add(key string, val interface{}) {
	utility.Add(e.data, key, val)
}
