package decoder

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests/loader"
)

func TestDecode(t *testing.T) {
	for _, ct := range []string{
		"application/json",
		"application/json; charset=UTF-8",
	} {
		input, err := loader.LoadValidData("transaction")
		assert.Nil(t, err)
		buffer := bytes.NewReader(input.Data)

		req, err := http.NewRequest("POST", "_", buffer)
		assert.Nil(t, err)

		req.Header.Add("Content-Type", ct)
		decoded, err := DecodeLimitJSONData(1024 * 1024)(req)
		assert.Nil(t, err)
		assert.Equal(t, input.Data, decoded.Data)
	}
}

func TestDecodeSizeLimit(t *testing.T) {
	minimalValid := func() *http.Request {
		req, err := http.NewRequest("POST", "_", strings.NewReader("{}"))
		assert.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")
		return req
	}

	// just fits
	_, err := DecodeLimitJSONData(2)(minimalValid())
	assert.Nil(t, err)

	// too large, should not be EOF
	_, err = DecodeLimitJSONData(1)(minimalValid())
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
	_, err := DecodeLimitJSONData(40)(gzipRequest(bigDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = DecodeLimitJSONData(1)(gzipRequest(bigDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)

	// compressed size > uncompressed (edge case)
	tinyData := "{}"
	tinyDataGz := gzipBody(tinyData)
	if len(tinyDataGz) < len(tinyData) {
		t.Fatal("compressed data unexpectedly small")
	}
	/// uncompressed just fits
	_, err = DecodeLimitJSONData(2)(gzipRequest(tinyDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = DecodeLimitJSONData(1)(gzipRequest(tinyDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)
}

func TestDecodeSourcemapFormData(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	smapData, err := loader.LoadData("", "data/valid/sourcemap/bundle.js.map")
	assert.NoError(t, err)
	fileBytes := smapData.Data
	part, err := writer.CreateFormFile("sourcemap", "bundle_no_mapping.js.map")
	assert.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(fileBytes))
	assert.NoError(t, err)

	writer.WriteField("bundle_filepath", "js/./test/../bundle_no_mapping.js.map")
	writer.WriteField("service_name", "My service")
	writer.WriteField("service_version", "0.1")

	err = writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest("POST", "_", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	assert.NoError(t, err)

	assert.NoError(t, err)
	data, err := DecodeSourcemapFormData(req)
	assert.NoError(t, err)

	assert.Equal(t, "js/bundle_no_mapping.js.map", data.BundleFilepath)
	assert.Equal(t, "My service", data.ServiceName)
	assert.Equal(t, "0.1", data.ServiceVersion)
	assert.NotNil(t, data.Data)
	assert.Equal(t, fileBytes, data.Data)
}

func TestDecodeSystemData(t *testing.T) {

	for _, enabled := range []bool{true, false} {
		req := transactionReq(t)
		body, err := DecodeSystemData(DecodeLimitJSONData(1024*1024), enabled)(req)
		assert.Nil(t, err)

		hasIp := body.SystemIP != ""
		assert.Equal(t, enabled, hasIp)
	}
}

func TestDecodeUserData(t *testing.T) {

	for _, enabled := range []bool{true, false} {
		req := transactionReq(t)
		body, err := DecodeUserData(DecodeLimitJSONData(1024*1024), enabled)(req)
		assert.Nil(t, err)

		hasIp := body.UserIP != ""
		assert.Equal(t, enabled, hasIp)

		hasUserAgent := body.UserAgent != ""
		assert.Equal(t, enabled, hasUserAgent)
	}
}

func transactionReq(t *testing.T) *http.Request {
	transaction, err := loader.LoadValidData("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transaction.Data)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "ruby-1.0")
	req.RemoteAddr = "10.11.12.13:8080"
	assert.Nil(t, err)
	return req
}
