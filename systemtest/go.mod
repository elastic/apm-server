module github.com/elastic/apm-server/systemtest

go 1.14

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200916142827-bd33bbf0497b+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/elastic/apm-server/approvaltest v0.0.0-00010101000000-000000000000
	github.com/elastic/go-elasticsearch/v7 v7.8.0
	github.com/elastic/go-sysinfo v1.4.0 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/google/pprof v0.0.0-20210406223550-17a10ee72223
	github.com/jaegertracing/jaeger v1.18.1
	github.com/mitchellh/mapstructure v1.1.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/prometheus/procfs v0.1.3 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.9.0
	github.com/tidwall/gjson v1.6.5
	github.com/tidwall/pretty v1.1.0 // indirect
	go.elastic.co/apm v1.8.1-0.20200913025752-7af7e1529586
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/otel v0.19.0
	go.opentelemetry.io/otel/exporters/otlp v0.19.0
	go.opentelemetry.io/otel/metric v0.19.0
	go.opentelemetry.io/otel/sdk v0.19.0
	go.opentelemetry.io/otel/sdk/export/metric v0.19.0
	go.opentelemetry.io/otel/sdk/metric v0.19.0
	go.opentelemetry.io/otel/trace v0.19.0
	go.uber.org/zap v1.15.0
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f
	google.golang.org/grpc v1.36.0
	gotest.tools v2.2.0+incompatible // indirect
	howett.net/plist v0.0.0-20200419221736-3b63eb3a43b5 // indirect
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/elastic/apm-server/approvaltest => ../approvaltest
)
