# Changelog

## [Unreleased](https://github.com/elastic/apm-agent-go/compare/v0.5.1...master)

## [v0.5.1](https://github.com/elastic/apm-agent-go/releases/tag/v0.5.1)

 - Fixed a bug causing error stacktraces and culprit to sometimes not be set (#204)

## [v0.5.0](https://github.com/elastic/apm-agent-go/releases/tag/v0.5.0)

 - `ELASTIC_APM_SERVER_URL` now defaults to "http://localhost:8200" (#122)
 - `Transport.SetUserAgent` method added, enabling the User-Agent to be set programatically (#124)
 - Inlined functions are now properly reported in stacktraces (#127)
 - Support for the experimental metrics API added (#94)
 - module/apmsql: SQL is parsed to generate more useful span names (#129)
 - Basic vgo module added (#136)
 - module/apmhttprouter: added a wrapper type for `httprouter.Router` to simplify adding routes (#140)
 - Add `Transaction.Context` methods for setting user IDs (#144)
 - module/apmgocql: new instrumentation module, providing an observer for gocql (#148)
 - Add `ELASTIC_APM_SERVER_TIMEOUT` config (#157)
 - Add `ELASTIC_APM_IGNORE_URLS` config (#158)
 - module/apmsql: fix a bug preventing errors from being captured (#160)
 - Introduce `Tracer.StartTransactionOptions`, drop variadic args from `Tracer.StartTransaction` (#165)
 - module/apmgorm: introduce GORM instrumentation module (#169, #170)
 - module/apmhttp: record outgoing request URLs in span context (#172)
 - module/apmot: introduce OpenTracing implementation (#173)

## [v0.4.0](https://github.com/elastic/apm-agent-go/releases/tag/v0.4.0)

First release of the Go agent for Elastic APM
