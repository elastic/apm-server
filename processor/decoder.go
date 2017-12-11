package processor

import (
	"net/http"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/utility"
)

func DecodeUserData(decoder decoder.Decoder) decoder.Decoder {
	augment := func(req *http.Request) map[string]interface{} {
		return map[string]interface{}{
			"ip":         utility.ExtractIP(req),
			"user_agent": req.Header.Get("User-Agent"),
		}
	}
	return augmentData(decoder, "user", augment)
}

func DecodeSystemData(decoder decoder.Decoder) decoder.Decoder {
	augment := func(req *http.Request) map[string]interface{} {
		return map[string]interface{}{"ip": utility.ExtractIP(req)}
	}
	return augmentData(decoder, "system", augment)
}

func augmentData(decoder decoder.Decoder, key string, augment func(req *http.Request) map[string]interface{}) decoder.Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		v, err := decoder(req)
		if err != nil {
			return v, err
		}
		utility.InsertInMap(v, key, augment(req))
		return v, nil
	}
}
