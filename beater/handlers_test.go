package beater

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"bytes"

	"github.com/stretchr/testify/assert"

	"encoding/json"

	"github.com/elastic/apm-server/tests"
)

func TestDecode(t *testing.T) {
	transactionBytes, err := tests.LoadValidData("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)
	var data map[string]interface{}
	json.Unmarshal(transactionBytes, &data)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	body, err := decodeLimitJSONData(1024 * 1024)(req)
	assert.Nil(t, err)
	assert.Equal(t, data, body)
}

func TestJSONFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)

	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`{"error":"Cannot compare apples to oranges"}`))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestJSONFailureResponseWhenAcceptingAnything(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`{"error":"Cannot compare apples to oranges"}`))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestHTMLFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`Cannot compare apples to oranges`))
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestFailureResponseNoAcceptHeader(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)

	req.Header.Del("Accept")

	w := httptest.NewRecorder()
	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`Cannot compare apples to oranges`))
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestIsAuthorized(t *testing.T) {
	reqAuth := func(auth string) *http.Request {
		req, err := http.NewRequest("POST", "_", nil)
		assert.Nil(t, err)
		req.Header.Add("Authorization", auth)
		return req
	}

	reqNoAuth, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)

	// Successes
	assert.True(t, isAuthorized(reqNoAuth, ""))
	assert.True(t, isAuthorized(reqAuth("foo"), ""))
	assert.True(t, isAuthorized(reqAuth("Bearer foo"), "foo"))

	// Failures
	assert.False(t, isAuthorized(reqNoAuth, "foo"))
	assert.False(t, isAuthorized(reqAuth("Bearer bar"), "foo"))
	assert.False(t, isAuthorized(reqAuth("Bearer foo extra"), "foo"))
	assert.False(t, isAuthorized(reqAuth("foo"), "foo"))
}

func TestExtractIP(t *testing.T) {
	var req = func(real *string, forward *string) *http.Request {
		req, _ := http.NewRequest("POST", "_", nil)
		req.RemoteAddr = "10.11.12.13:8080"
		if real != nil {
			req.Header.Add("X-Real-IP", *real)
		}
		if forward != nil {
			req.Header.Add("X-Forwarded-For", *forward)
		}
		return req
	}

	real := "54.55.101.102"
	assert.Equal(t, real, extractIP(req(&real, nil)))

	forwardedFor := "54.56.103.104"
	assert.Equal(t, real, extractIP(req(&real, &forwardedFor)))
	assert.Equal(t, forwardedFor, extractIP(req(nil, &forwardedFor)))

	forwardedForMultiple := "54.56.103.104 , 54.57.105.106 , 54.58.107.108"
	assert.Equal(t, forwardedFor, extractIP(req(nil, &forwardedForMultiple)))

	assert.Equal(t, "10.11.12.13", extractIP(req(nil, nil)))
	assert.Equal(t, "10.11.12.13", extractIP(req(new(string), new(string))))
}
