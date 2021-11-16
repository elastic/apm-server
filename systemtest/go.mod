module github.com/elastic/apm-server/systemtest

go 1.14

require (
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v20.10.10+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/elastic/apm-server/approvaltest v0.0.0-00010101000000-000000000000
	github.com/elastic/go-elasticsearch/v7 v7.8.0
	github.com/elastic/go-sysinfo v1.7.0 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/gofrs/uuid v4.1.0+incompatible
	github.com/google/pprof v0.0.0-20210406223550-17a10ee72223
	github.com/jaegertracing/jaeger v1.18.1
	github.com/mitchellh/mapstructure v1.1.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/prometheus/procfs v0.7.1 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.11.2-0.20211110075312-4b5710b46477
	github.com/tidwall/gjson v1.9.3
	go.elastic.co/apm v1.14.1-0.20211027055810-a8ec1811e727
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/otel v1.0.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.23.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.23.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.0.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.0.0
	go.opentelemetry.io/otel/metric v0.23.0
	go.opentelemetry.io/otel/sdk v1.0.0
	go.opentelemetry.io/otel/sdk/export/metric v0.23.0
	go.opentelemetry.io/otel/sdk/metric v0.23.0
	go.opentelemetry.io/otel/trace v1.0.0
	go.uber.org/zap v1.15.0
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211109184856-51b60fd695b3
	google.golang.org/grpc v1.40.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

require (
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	golang.org/x/tools v0.1.5 // indirect
	howett.net/plist v0.0.0-20201203080718-1454fab16a06 // indirect
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.4.11
	github.com/elastic/apm-server/approvaltest => ../approvaltest
)
