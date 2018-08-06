package beater

import (
	"net/http"
	"regexp"

	"github.com/elastic/apm-server/processor"
	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

type routeType struct {
	wrappingHandler     func(*Config, http.Handler) http.Handler
	configurableDecoder func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	transformConfig     func(*Config) transform.Config
}

type v1Route struct {
	routeType
	processor.Processor
}

type v2Route struct {
	routeType
}

var v1Routes = map[string]v1Route{
	BackendTransactionsURL:    {backendRouteType, transaction.Processor},
	ClientSideTransactionsURL: {rumRouteType, transaction.Processor},
	RumTransactionsURL:        {rumRouteType, transaction.Processor},
	BackendErrorsURL:          {backendRouteType, perr.Processor},
	ClientSideErrorsURL:       {rumRouteType, perr.Processor},
	RumErrorsURL:              {rumRouteType, perr.Processor},
	MetricsURL:                {metricsRouteType, metric.Processor},
	SourcemapsClientSideURL:   {sourcemapRouteType, sourcemap.Processor},
	SourcemapsURL:             {sourcemapRouteType, sourcemap.Processor},
}

var (
	backendRouteType = routeType{
		backendHandler,
		backendMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	}
	rumRouteType = routeType{
		rumHandler,
		rumMetadataDecoder,
		rumTransformConfig,
	}
	metricsRouteType = routeType{
		metricsHandler,
		backendMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	}
	sourcemapRouteType = routeType{
		sourcemapHandler,
		backendMetadataDecoder,
		rumTransformConfig,
	}
)

var v2Routes = map[string]v2Route{
	"/v2/intake": {
		backendRouteType,
	},
	"/v2/rum/intake": {
		rumRouteType,
	},
}

func backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		concurrencyLimitHandler(beaterConfig,
			authHandler(beaterConfig.SecretToken, h)))
}

func rumHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
		concurrencyLimitHandler(beaterConfig,
			ipRateLimitHandler(beaterConfig.RumConfig.RateLimit,
				corsHandler(beaterConfig.RumConfig.AllowOrigins, h))))
}

func metricsHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Metrics.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func sourcemapHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func backendMetadataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeSystemData(d, beaterConfig.AugmentEnabled)
}

func rumMetadataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeUserData(d, beaterConfig.AugmentEnabled)
}

func rumTransformConfig(beaterConfig *Config) transform.Config {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	config := transform.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}
	return config
}
