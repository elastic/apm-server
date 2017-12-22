package sourcemap

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func DecodeSourcemapFormData(req *http.Request) (map[string]interface{}, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	file, _, err := req.FormFile("sourcemap")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sourcemap bytes.Buffer
	_, err = io.Copy(&sourcemap, file)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"sourcemap":       sourcemap.Bytes(),
		"service_name":    req.FormValue("service_name"),
		"service_version": req.FormValue("service_version"),
		"bundle_filepath": req.FormValue("bundle_filepath"),
	}

	return payload, nil
}
