package sourcemap

import (
	"bytes"
	"encoding/json"
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

	fileBytes, err := tests.LoadData("tests/data/valid/sourcemap/bundle.min.map")
	assert.NoError(t, err)
	part, err := writer.CreateFormFile("sourcemap", "bundle.min.map")
	assert.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(fileBytes))
	assert.NoError(t, err)

	writer.WriteField("bundle_filepath", "js/bundle.min.map")
	writer.WriteField("service_name", "My service")
	writer.WriteField("service_version", "0.1")

	err = writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest("POST", "_", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	assert.NoError(t, err)

	assert.NoError(t, err)
	buf, err := DecodeSourcemapFormData(req)
	assert.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal(buf, &data)
	assert.NoError(t, err)

	assert.Len(t, data, 4)
	assert.Equal(t, "js/bundle.min.map", data["bundle_filepath"])
	assert.Equal(t, "My service", data["service_name"])
	assert.Equal(t, "0.1", data["service_version"])

	for _, k := range []string{"version", "sources", "names", "mappings", "file", "sourcesContent", "sourceRoot"} {
		assert.Contains(t, data["sourcemap"], k, data)
		assert.NotNil(t, data["sourcemap"].(map[string]interface{})[k])
	}
}
