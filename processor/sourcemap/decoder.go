package sourcemap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/utility"
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

	var parsedSourcemap map[string]interface{}
	err = json.NewDecoder(file).Decode(&parsedSourcemap)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"sourcemap":       parsedSourcemap,
		"service_name":    req.FormValue("service_name"),
		"service_version": req.FormValue("service_version"),
		"bundle_filepath": utility.CleanUrlPath(req.FormValue("bundle_filepath")),
	}

	return payload, nil
}
