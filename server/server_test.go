package server

import (
	"bytes"
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
	req, err := http.NewRequest("POST", "/transactions", buffer)
	req.Header.Add("Content-Type", "application/json")
	res, err := decodeData(req)
	assert.Nil(t, err)
	assert.NotNil(t, res)
}

func TestJSONFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	sendError(w, req, 400, "Cannot compare apples to oranges", false)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`{"error":"Cannot compare apples to oranges"}`))
}

func TestJSONFailureResponseWhenAcceptingAnything(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	sendError(w, req, 400, "Cannot compare apples to oranges", false)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`{"error":"Cannot compare apples to oranges"}`))
}

func TestHTMLFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	sendError(w, req, 400, "Cannot compare apples to oranges", false)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`Cannot compare apples to oranges`))
}

func TestFailureResponseNoAcceptHeader(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)

	req.Header.Del("Accept")

	w := httptest.NewRecorder()
	sendError(w, req, 400, "Cannot compare apples to oranges", false)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`Cannot compare apples to oranges`))
}
