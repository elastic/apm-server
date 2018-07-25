package beater

import (
	"net/http"
	"regexp"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

func backendHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	return logHandler(
		concurrencyLimitHandler(beaterConfig,
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(p, transform.Config{}, report,
					decoder.DecodeSystemData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))
}

func rumHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	config := transform.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			concurrencyLimitHandler(beaterConfig,
				ipRateLimitHandler(beaterConfig.RumConfig.RateLimit,
					corsHandler(beaterConfig.RumConfig.AllowOrigins,
						processRequestHandler(p, config, report,
							decoder.DecodeUserData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))))
}

func metricsHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Metrics.isEnabled(),
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(p, transform.Config{}, report,
					decoder.DecodeSystemData(decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize), beaterConfig.AugmentEnabled)))))
}

func sourcemapHandler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			authHandler(beaterConfig.SecretToken,
				processRequestHandler(p, transform.Config{SmapMapper: smapper}, report, decoder.DecodeSourcemapFormData))))
}
