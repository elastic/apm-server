package sourcemap

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
)

func TestDecodeSourcemapFormData(t *testing.T) {

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileBytes, err := tests.LoadDataAsBytes("data/valid/sourcemap/bundle.js.map")
	assert.NoError(t, err)
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

	assert.Len(t, data, 4)
	assert.Equal(t, "js/bundle_no_mapping.js.map", data["bundle_filepath"])
	assert.Equal(t, "My service", data["service_name"])
	assert.Equal(t, "0.1", data["service_version"])
	assert.NotNil(t, data["sourcemap"].(string))
	assert.Equal(t, len(fileBytes), len(data["sourcemap"].(string)))
}
