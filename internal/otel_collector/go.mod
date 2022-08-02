module go.opentelemetry.io/collector

go 1.17

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.1
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v0.0.4
	github.com/google/uuid v1.3.0
	github.com/klauspost/compress v1.15.8
	github.com/knadh/koanf v1.4.2
	github.com/magiconair/properties v1.8.6
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mostynb/go-grpc-compression v1.1.16
	github.com/prometheus/common v0.37.0
	github.com/rs/cors v1.8.2
	github.com/shirou/gopsutil/v3 v3.22.6
	github.com/spf13/cast v1.5.0
	github.com/spf13/cobra v1.5.0
	github.com/stretchr/testify v1.8.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector/pdata v0.56.0
	go.opentelemetry.io/collector/semconv v0.56.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.33.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.33.0
	go.opentelemetry.io/contrib/zpages v0.33.0
	go.opentelemetry.io/otel v1.8.0
	go.opentelemetry.io/otel/exporters/prometheus v0.31.0
	go.opentelemetry.io/otel/metric v0.31.0
	go.opentelemetry.io/otel/sdk v1.8.0
	go.opentelemetry.io/otel/sdk/metric v0.31.0
	go.opentelemetry.io/otel/trace v1.8.0
	go.uber.org/atomic v1.9.0
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa
	google.golang.org/grpc v1.48.0
	google.golang.org/protobuf v1.28.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.12.2 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace go.opentelemetry.io/collector/semconv => ./semconv

replace go.opentelemetry.io/collector/pdata => ./pdata

retract v0.32.0 // Contains incomplete metrics transition to proto 0.9.0, random components are not working.
