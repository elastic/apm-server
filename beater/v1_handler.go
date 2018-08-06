package beater

import (
	"net/http"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor"
)

func (v *v1Route) Handler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	decoder := v.configurableDecoder(beaterConfig, decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize))
	tconfig := v.transformConfig(beaterConfig)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := processRequest(r, p, tconfig, report, decoder)
		sendStatus(w, r, res)
	})

	return v.wrappingHandler(beaterConfig, handler)
}
