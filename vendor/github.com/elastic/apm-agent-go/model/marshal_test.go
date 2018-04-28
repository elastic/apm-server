package model_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

func TestMarshalTransaction(t *testing.T) {
	tx := fakeTransaction()

	var w fastjson.Writer
	tx.MarshalFastJSON(&w)

	var in map[string]interface{}
	if err := json.Unmarshal(w.Bytes(), &in); err != nil {
		t.Fatalf("unmarshalling result failed: %v", err)
	}

	expect := map[string]interface{}{
		"id":        "d51ae41d-93da-4984-bba3-ae15e9b2247f",
		"name":      "GET /foo/bar",
		"type":      "request",
		"timestamp": "1970-01-01T00:02:03Z",
		"duration":  123.456,
		"result":    "418",
		"context": map[string]interface{}{
			"request": map[string]interface{}{
				"url": map[string]interface{}{
					"full":     "https://testing.invalid/foo/bar?baz#qux",
					"protocol": "https",
					"hostname": "testing.invalid",
					"pathname": "/foo/bar",
					"search":   "baz",
					"hash":     "qux",
				},
				"method": "GET",
				"headers": map[string]interface{}{
					"user-agent": "Mosaic/0.2 (Windows 3.1)",
					"cookie":     "monster=yumyum; random=junk",
				},
				"body":         "ahoj",
				"http_version": "1.1",
				"cookies": map[string]interface{}{
					"monster": "yumyum",
					"random":  "junk",
				},
				"socket": map[string]interface{}{
					"encrypted":      true,
					"remote_address": "[::1]",
				},
			},
			"response": map[string]interface{}{
				"status_code": float64(418),
				"headers": map[string]interface{}{
					"content-type": "text/html",
				},
			},
			"user": map[string]interface{}{
				"username": "wanda",
			},
			"custom": map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
					"qux": float64(123),
				},
			},
			"tags": map[string]interface{}{
				"tag": "urit",
			},
		},
		"span_count": map[string]interface{}{
			"dropped": map[string]interface{}{
				"total": float64(4),
			},
		},
		"spans": []interface{}{
			map[string]interface{}{
				"name":     "SELECT FROM bar",
				"start":    float64(2),
				"duration": float64(3),
				"type":     "db.postgresql.query",
				"context": map[string]interface{}{
					"db": map[string]interface{}{
						"instance":  "wat",
						"statement": `SELECT foo FROM bar WHERE baz LIKE 'qu%x'`,
						"type":      "sql",
						"user":      "barb",
					},
				},
			},
		},
	}

	assert.Equal(t, expect, in)
}

func TestMarshalPayloads(t *testing.T) {
	tp := fakeTransactionsPayload(0)
	var w fastjson.Writer
	tp.MarshalFastJSON(&w)

	var in map[string]interface{}
	if err := json.Unmarshal(w.Bytes(), &in); err != nil {
		t.Fatalf("unmarshalling result failed: %v", err)
	}

	expect := map[string]interface{}{
		"process": map[string]interface{}{
			"pid":   float64(1234),
			"ppid":  float64(1),
			"title": "my-fake-service",
			"argv":  []interface{}{"my-fake-service", "-f", "config.yml"},
		},
		"service": map[string]interface{}{
			"environment": "dev",
			"agent": map[string]interface{}{
				"name":    "go",
				"version": "0.1.0",
			},
			"framework": map[string]interface{}{
				"name":    "gin",
				"version": "1.0",
			},
			"language": map[string]interface{}{
				"name":    "go",
				"version": "1.10",
			},
			"runtime": map[string]interface{}{
				"name":    "go",
				"version": "gc 1.10",
			},
			"name":    "fake-service",
			"version": "1.0.0-rc1",
		},
		"system": map[string]interface{}{
			"architecture": "x86_64",
			"hostname":     "host.example",
			"platform":     "linux",
		},
		"transactions": []interface{}{},
	}
	assert.Equal(t, expect, in)

	ep := &model.ErrorsPayload{
		Service: tp.Service,
		Process: tp.Process,
		System:  tp.System,
		Errors:  []*model.Error{},
	}
	w.Reset()
	ep.MarshalFastJSON(&w)

	in = make(map[string]interface{})
	if err := json.Unmarshal(w.Bytes(), &in); err != nil {
		t.Fatalf("unmarshalling result failed: %v", err)
	}
	delete(expect, "transactions")
	expect["errors"] = []interface{}{}
	assert.Equal(t, expect, in)
}

func TestMarshalError(t *testing.T) {
	var e model.Error
	time, err := time.Parse("2006-01-02T15:04:05.999Z", "1970-01-01T00:02:03Z")
	assert.NoError(t, err)
	e.Timestamp = model.Time(time)

	var w fastjson.Writer
	e.MarshalFastJSON(&w)
	assert.Equal(t, `{"timestamp":"1970-01-01T00:02:03Z"}`, string(w.Bytes()))

	e.Transaction.ID = "xyz"
	w.Reset()
	e.MarshalFastJSON(&w)
	assert.Equal(t, `{"timestamp":"1970-01-01T00:02:03Z","transaction":{"id":"xyz"}}`, string(w.Bytes()))
}

func TestMarshalCookies(t *testing.T) {
	cookies := model.Cookies{
		{Name: "foo", Value: "!"}, // eclipsed
		{Name: "baz", Value: "qux"},
		{Name: "foo", Value: "bar"},
	}
	var w fastjson.Writer
	cookies.MarshalFastJSON(&w)
	assert.Equal(t, `{"foo":"bar","baz":"qux"}`, string(w.Bytes()))
}

func TestMarshalRequestBody(t *testing.T) {
	body := model.RequestBody{
		Raw: "rawr",
	}
	var w fastjson.Writer
	body.MarshalFastJSON(&w)
	assert.Equal(t, `"rawr"`, string(w.Bytes()))

	body.Form = url.Values{
		"first":    []string{"jackie"},
		"last":     []string{"brown"},
		"keywords": []string{"rum", "punch"},
	}
	w.Reset()
	body.MarshalFastJSON(&w)

	var in map[string]interface{}
	if err := json.Unmarshal(w.Bytes(), &in); err != nil {
		t.Fatalf("unmarshalling result failed: %v", err)
	}
	expect := map[string]interface{}{
		"first":    "jackie",
		"last":     "brown",
		"keywords": []interface{}{"rum", "punch"},
	}
	assert.Equal(t, expect, in)
}

func TestMarshalLog(t *testing.T) {
	log := model.Log{
		Message:      "foo",
		Level:        "bar",
		LoggerName:   "baz",
		ParamMessage: "%s",
	}
	var w fastjson.Writer
	log.MarshalFastJSON(&w)

	assert.Equal(t, `{"message":"foo","level":"bar","logger_name":"baz","param_message":"%s"}`, string(w.Bytes()))

	log = model.Log{
		Message:    "foo",
		LoggerName: "bar",
	}
	w.Reset()
	log.MarshalFastJSON(&w)
	assert.Equal(t, `{"message":"foo","logger_name":"bar"}`, string(w.Bytes()))
}

func TestMarshalException(t *testing.T) {
	x := model.Exception{
		Message: "foo",
		Type:    "bar",
		Module:  "baz",
		Attributes: map[string]interface{}{
			"qux": map[string]interface{}{
				"quux": "corge",
			},
		},
		Handled: true,
	}
	var w fastjson.Writer
	x.MarshalFastJSON(&w)

	assert.Equal(t,
		`{"handled":true,"message":"foo","attributes":{"qux":{"quux":"corge"}},"module":"baz","type":"bar"}`,
		string(w.Bytes()),
	)
}

func TestMarshalExceptionCode(t *testing.T) {
	code := model.ExceptionCode{
		String: "boom",
		Number: 123,
	}
	var w fastjson.Writer
	code.MarshalFastJSON(&w)
	assert.Equal(t, `"boom"`, string(w.Bytes()))

	w.Reset()
	code.String = ""
	code.MarshalFastJSON(&w)
	assert.Equal(t, `123`, string(w.Bytes()))
}

func TestMarshalUser(t *testing.T) {
	user := model.User{
		Email:    "foo@example.com",
		ID:       123,
		Username: "bar",
	}
	var w fastjson.Writer
	user.MarshalFastJSON(&w)
	assert.Equal(t, `{"email":"foo@example.com","id":123,"username":"bar"}`, string(w.Bytes()))
}

func TestMarshalStacktraceFrame(t *testing.T) {
	f := model.StacktraceFrame{
		File:         "file.go",
		Line:         123,
		AbsolutePath: "fabulous",
		Function:     "wonderment",
	}
	var w fastjson.Writer
	f.MarshalFastJSON(&w)

	assert.Equal(t,
		`{"filename":"file.go","lineno":123,"abs_path":"fabulous","function":"wonderment"}`,
		string(w.Bytes()),
	)

	f = model.StacktraceFrame{
		File:         "file.go",
		Line:         123,
		LibraryFrame: true,
		ContextLine:  "0",
		PreContext:   []string{"-2", "-1"},
		PostContext:  []string{"+1", "+2"},
		Vars: map[string]interface{}{
			"foo": []string{"bar", "baz"},
		},
	}
	w.Reset()
	f.MarshalFastJSON(&w)
	assert.Equal(t,
		`{"filename":"file.go","lineno":123,"context_line":"0","library_frame":true,"post_context":["+1","+2"],"pre_context":["-2","-1"],"vars":{"foo":["bar","baz"]}}`,
		string(w.Bytes()),
	)
}

func TestMarshalContextCustomErrors(t *testing.T) {
	context := model.Context{
		Custom: model.IfaceMap{{
			Key: "panic_value",
			Value: marshalFunc(func() ([]byte, error) {
				panic("aiee")
			}),
		}},
	}
	var w fastjson.Writer
	context.MarshalFastJSON(&w)
	assert.Equal(t,
		`{"custom":{"panic_value":{"__PANIC__":"panic calling MarshalJSON for type model_test.marshalFunc: aiee"}}}`,
		string(w.Bytes()),
	)

	context.Custom = model.IfaceMap{{
		Key: "error_value",
		Value: marshalFunc(func() ([]byte, error) {
			return nil, errors.New("nope")
		}),
	}}
	w.Reset()
	context.MarshalFastJSON(&w)
	assert.Equal(t,
		`{"custom":{"error_value":{"__ERROR__":"json: error calling MarshalJSON for type model_test.marshalFunc: nope"}}}`,
		string(w.Bytes()),
	)
}

type marshalFunc func() ([]byte, error)

func (f marshalFunc) MarshalJSON() ([]byte, error) {
	return f()
}

func TestMarshalCustomInvalidJSON(t *testing.T) {
	context := model.Context{
		Custom: model.IfaceMap{{
			Key: "k1",
			Value: appenderFunc(func(in []byte) []byte {
				return append(in, "123"...)
			}),
		}, {
			Key: "k2",
			Value: appenderFunc(func(in []byte) []byte {
				return append(in, `"value"`...)
			}),
		}},
	}
	var w fastjson.Writer
	context.MarshalFastJSON(&w)
	assert.Equal(t, `{"custom":{"k1":123,"k2":"value"}}`, string(w.Bytes()))
}

type appenderFunc func([]byte) []byte

func (f appenderFunc) AppendJSON(in []byte) []byte {
	return f(in)
}

func TestMarshalResponse(t *testing.T) {
	finished := true
	headersSent := true
	response := model.Response{
		Finished: &finished,
		Headers: &model.ResponseHeaders{
			ContentType: "text/plain",
		},
		HeadersSent: &headersSent,
		StatusCode:  200,
	}
	var w fastjson.Writer
	response.MarshalFastJSON(&w)
	assert.Equal(t,
		`{"finished":true,"headers":{"content-type":"text/plain"},"headers_sent":true,"status_code":200}`,
		string(w.Bytes()),
	)
}

func TestUnmarshalJSON(t *testing.T) {
	tp := fakeTransactionsPayload(1)
	var w fastjson.Writer
	tp.MarshalFastJSON(&w)

	var out model.TransactionsPayload
	err := json.Unmarshal(w.Bytes(), &out)
	require.NoError(t, err)
	assert.Equal(t, &tp, &out)
}

func fakeTransaction() *model.Transaction {
	return &model.Transaction{
		ID:        "d51ae41d-93da-4984-bba3-ae15e9b2247f",
		Name:      "GET /foo/bar",
		Type:      "request",
		Timestamp: model.Time(time.Unix(123, 0).UTC()),
		Duration:  123.456,
		Result:    "418",
		Context: &model.Context{
			Request: &model.Request{
				URL: model.URL{
					Full:     "https://testing.invalid/foo/bar?baz#qux",
					Hostname: "testing.invalid",
					Protocol: "https",
					Path:     "/foo/bar",
					Search:   "baz",
					Hash:     "qux",
				},
				Method: "GET",
				Headers: &model.RequestHeaders{
					UserAgent: "Mosaic/0.2 (Windows 3.1)",
					Cookie:    "monster=yumyum; random=junk",
				},
				Body: &model.RequestBody{
					Raw: "ahoj",
				},
				HTTPVersion: "1.1",
				Cookies: []*http.Cookie{
					{Name: "monster", Value: "yumyum"},
					{Name: "random", Value: "junk"},
				},
				Socket: &model.RequestSocket{
					Encrypted:     true,
					RemoteAddress: "[::1]",
				},
			},
			Response: &model.Response{
				StatusCode: 418,
				Headers: &model.ResponseHeaders{
					ContentType: "text/html",
				},
			},
			User: &model.User{
				Username: "wanda",
			},
			Custom: model.IfaceMap{{
				Key: "foo",
				Value: map[string]interface{}{
					"bar": "baz",
					"qux": float64(123),
				},
			}},
			Tags: map[string]string{
				"tag": "urit",
			},
		},
		SpanCount: model.SpanCount{
			Dropped: model.SpanCountDropped{
				Total: 4,
			},
		},
		Spans: []*model.Span{{
			Name:     "SELECT FROM bar",
			Start:    2,
			Duration: 3,
			Type:     "db.postgresql.query",
			Context: &model.SpanContext{
				Database: &model.DatabaseSpanContext{
					Instance:  "wat",
					Statement: `SELECT foo FROM bar WHERE baz LIKE 'qu%x'`,
					Type:      "sql",
					User:      "barb",
				},
			},
		}},
	}
}

func fakeService() *model.Service {
	return &model.Service{
		Name:        "fake-service",
		Version:     "1.0.0-rc1",
		Environment: "dev",
		Agent: model.Agent{
			Name:    "go",
			Version: "0.1.0",
		},
		Framework: &model.Framework{
			Name:    "gin",
			Version: "1.0",
		},
		Language: &model.Language{
			Name:    "go",
			Version: "1.10",
		},
		Runtime: &model.Runtime{
			Name:    "go",
			Version: "gc 1.10",
		},
	}
}

func fakeSystem() *model.System {
	return &model.System{
		Architecture: "x86_64",
		Hostname:     "host.example",
		Platform:     "linux",
	}
}

func fakeProcess() *model.Process {
	ppid := 1
	return &model.Process{
		Pid:   1234,
		Ppid:  &ppid,
		Title: "my-fake-service",
		Argv:  []string{"my-fake-service", "-f", "config.yml"},
	}
}

func fakeTransactionsPayload(n int) model.TransactionsPayload {
	transactions := make([]*model.Transaction, n)
	tx := fakeTransaction()
	for i := range transactions {
		transactions[i] = tx
	}
	return model.TransactionsPayload{
		Service:      fakeService(),
		Process:      fakeProcess(),
		System:       fakeSystem(),
		Transactions: transactions,
	}
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
