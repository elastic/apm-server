package sourcemap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func DecodeSourcemapFormData(req *http.Request) ([]byte, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	file, _, err := req.FormFile("sourcemap")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var parsedSourcemap map[string]interface{}
	err = json.NewDecoder(file).Decode(&parsedSourcemap)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"sourcemap":       parsedSourcemap,
		"app_name":        req.FormValue("app_name"),
		"app_version":     req.FormValue("app_version"),
		"bundle_filepath": req.FormValue("bundle_filepath"),
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
