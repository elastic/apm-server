// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package apmservertest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

// LogEntries holds apm-server log entries.
type LogEntries struct {
	closed   chan struct{}
	commands chan func()

	mu    sync.RWMutex
	logs  []LogEntry
	added *sync.Cond
}

func (logs *LogEntries) init() {
	logs.closed = make(chan struct{})
	logs.commands = make(chan func())
	logs.added = sync.NewCond(logs.mu.RLocker())
	go logs.loop()
}

func (logs *LogEntries) close() {
	closeCommand := func() {
		logs.mu.Lock()
		defer logs.mu.Unlock()
		close(logs.closed)
		logs.added.Broadcast()
	}
	select {
	case <-logs.closed:
	case logs.commands <- closeCommand:
	}
}

func (logs *LogEntries) loop() {
	defer logs.added.Broadcast()
	for {
		select {
		case <-logs.closed:
			return
		case cmd := <-logs.commands:
			cmd()
		}
	}
}

func (logs *LogEntries) add(e LogEntry) {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	logs.logs = append(logs.logs, e)
	logs.added.Broadcast()
}

// All returns all log entries captured so far.
func (logs *LogEntries) All() []LogEntry {
	logs.mu.RLock()
	defer logs.mu.RUnlock()
	return logs.logs
}

// wait waits until there are logs available from the index 'pos',
// the abort channel is signalled, or logs is closed.
func (logs *LogEntries) wait(pos int, abort <-chan struct{}) ([]LogEntry, bool) {
	logs.mu.RLock()
	defer logs.mu.RUnlock()
	for {
		entries := logs.logs[pos:]
		if len(entries) > 0 {
			return entries, true
		}
		select {
		case <-abort:
			return nil, false
		case <-logs.closed:
			return nil, false
		default:
			logs.added.Wait()
		}
	}
}

// Iterator returns a new LogEntryIterator for iterating over logs. The
// iterator will remain alive until all existing logs have been consumed
// and logs is closed.
func (logs *LogEntries) Iterator() *LogEntryIterator {
	var once sync.Once
	done := make(chan struct{})
	ch := make(chan LogEntry)
	iter := &LogEntryIterator{
		ch:   ch,
		stop: func() { once.Do(func() { close(done) }) },
	}
	go func() {
		defer close(ch)
		var pos int
		for {
			entries, ok := logs.wait(pos, done)
			if !ok {
				return
			}
			for _, entry := range entries {
				select {
				case <-done:
					return
				case ch <- entry:
				}
			}
			pos += len(entries)
		}
	}()
	return iter
}

// LogEntryIterator provides a means of iterating over log entries.
type LogEntryIterator struct {
	ch   <-chan LogEntry
	stop func()
}

// C is a channel to which log entries will be sent. When the iterator
// is closed or the apm-server process exits, the channel will be closed.
func (iter *LogEntryIterator) C() <-chan LogEntry {
	return iter.ch
}

// Close closes the iterator, closing the channel returned by C().
func (iter *LogEntryIterator) Close() {
	iter.stop()
}

// LogEntry holds the details of an apm-server log entry.
type LogEntry struct {
	Timestamp time.Time
	Level     zapcore.Level
	Logger    string
	File      string
	Line      int
	Message   string
	Fields    map[string]interface{}
}

type logpTimestamp time.Time

func (t *logpTimestamp) UnmarshalText(text []byte) error {
	const ISO8601Layout = "2006-01-02T15:04:05.000Z0700"
	time, err := time.Parse(ISO8601Layout, string(text))
	if err != nil {
		return err
	}
	*t = logpTimestamp(time)
	return nil
}

// createLogfile creates a log file in apm-server/systemtest/logs,
// under a sub-directory named after the test, and with the given
// filename.
func createLogfile(tb testing.TB, filename string) *os.File {
	f, err := os.Create(filepath.Join(getTestLogsDir(tb), filename))
	if err != nil {
		tb.Fatal(err)
	}
	return f
}

func getTestLogsDir(tb testing.TB) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller(0) failed")
	}
	systemtestdir := filepath.Dir(filepath.Dir(file))
	logsdir := filepath.Join(systemtestdir, "logs")

	name := strings.ReplaceAll(tb.Name(), "/", "_")
	logsdir = filepath.Join(logsdir, name)
	if err := os.MkdirAll(logsdir, 0755); err != nil {
		tb.Fatal(err)
	}
	return logsdir
}
