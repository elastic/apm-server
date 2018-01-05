package beater

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
)

func TestDecode(t *testing.T) {
	transactionBytes, err := tests.LoadValidDataAsBytes("transaction")
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

func TestDecodeSizeLimit(t *testing.T) {
	minimalValid := func() *http.Request {
		req, err := http.NewRequest("POST", "_", strings.NewReader("{}"))
		assert.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")
		return req
	}

	// just fits
	_, err := decodeLimitJSONData(2)(minimalValid())
	assert.Nil(t, err)

	// too large, should not be EOF
	_, err = decodeLimitJSONData(1)(minimalValid())
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)
}

func TestDecodeSizeLimitGzip(t *testing.T) {
	gzipBody := func(body string) []byte {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte("{}"))
		assert.Nil(t, err)
		err = zw.Close()
		assert.Nil(t, err)
		return buf.Bytes()
	}
	gzipRequest := func(body []byte) *http.Request {
		req, err := http.NewRequest("POST", "_", bytes.NewReader(body))
		assert.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Content-Encoding", "gzip")
		return req
	}

	// compressed size < uncompressed (usual case)
	bigData := `{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 1}`
	bigDataGz := gzipBody(bigData)
	if len(bigDataGz) > len(bigData) {
		t.Fatal("compressed data unexpectedly big")
	}
	/// uncompressed just fits
	_, err := decodeLimitJSONData(40)(gzipRequest(bigDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = decodeLimitJSONData(1)(gzipRequest(bigDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)

	// compressed size > uncompressed (edge case)
	tinyData := "{}"
	tinyDataGz := gzipBody(tinyData)
	if len(tinyDataGz) < len(tinyData) {
		t.Fatal("compressed data unexpectedly small")
	}
	/// uncompressed just fits
	_, err = decodeLimitJSONData(2)(gzipRequest(tinyDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = decodeLimitJSONData(1)(gzipRequest(tinyDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)
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
