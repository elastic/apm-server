package beater

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncCounter(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	sendStatus(w, req, http.StatusServiceUnavailable, errFull)
	assert.Equal(t, int64(1), errMap[5030].Get())

	sendStatus(w, req, http.StatusServiceUnavailable, errConcurrencyLimitReached)
	assert.Equal(t, int64(1), errMap[5031].Get())

	sendStatus(w, req, http.StatusServiceUnavailable, errChannelClosed)
	assert.Equal(t, int64(1), errMap[5032].Get())

	sendStatus(w, req, http.StatusMethodNotAllowed, errors.New(""))
	assert.Equal(t, int64(1), errMap[405].Get())

	sendStatus(w, req, http.StatusTooManyRequests, errors.New(""))
	assert.Equal(t, int64(1), errMap[429].Get())

	sendStatus(w, req, http.StatusUnauthorized, errors.New(""))
	assert.Equal(t, int64(1), errMap[401].Get())

	sendStatus(w, req, http.StatusForbidden, errors.New(""))
	assert.Equal(t, int64(1), errMap[403].Get())

	sendStatus(w, req, http.StatusBadRequest, errors.New(""))
	assert.Equal(t, int64(1), errMap[400].Get())

	assert.Equal(t, int64(8), responseErrors.Get())

	responseErrors.Set(0)
	sendStatus(w, req, http.StatusNotImplemented, errors.New(""))
	sendStatus(w, req, http.StatusServiceUnavailable, errors.New(""))
	assert.Equal(t, int64(2), responseErrors.Get())

	responseErrors.Set(0)
	sendStatus(w, req, http.StatusOK, nil)
	assert.Equal(t, int64(0), responseErrors.Get())
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
