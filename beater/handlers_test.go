package beater

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
)

func TestDecode(t *testing.T) {
	transactionBytes, err := tests.LoadValidData("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	res, err := decodeData(req)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	body, err := ioutil.ReadAll(res)
	assert.Nil(t, err)
	assert.Equal(t, transactionBytes, body)
}

func TestJSONFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
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
	req, err := http.NewRequest("POST", "/transactions", nil)
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
	req, err := http.NewRequest("POST", "/transactions", nil)
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
	req, err := http.NewRequest("POST", "/transactions", nil)
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
		req, err := http.NewRequest("POST", "/", nil)
		assert.Nil(t, err)
		req.Header.Add("Authorization", auth)
		return req
	}

	reqNoAuth, err := http.NewRequest("POST", "/", nil)
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
