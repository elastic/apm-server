module github.com/elastic/apm-server

go 1.13

require (
	github.com/akavel/rsrc v0.9.0 // indirect
	github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/client9/misspell v0.3.5-0.20180309020325-c0b55c823952 // indirect
	github.com/dlclark/regexp2 v1.2.0 // indirect
	github.com/dop251/goja v0.0.0-20200629185240-bfd59704b500 // indirect
	github.com/dop251/goja_nodejs v0.0.0-20200629185815-2c377ae209ab // indirect
	github.com/dustin/go-humanize v0.0.0-20171111073723-bb3d318650d4
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20200702101253-794ed81680a9
	github.com/elastic/go-elasticsearch/v7 v7.5.0
	github.com/elastic/go-elasticsearch/v8 v8.0.0-20200210103600-aff00e5adfde
	github.com/elastic/go-hdrhistogram v0.1.0
	github.com/elastic/go-licenser v0.3.1
	github.com/elastic/go-ucfg v0.8.3
	github.com/fatih/color v1.9.0
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/gofrs/uuid v3.3.0+incompatible
	github.com/gogo/googleapis v1.3.1-0.20190914144012-b8d18e97a9a1 // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/google/pprof v0.0.0-20191218002539-d4f498aebedc
	github.com/hashicorp/golang-lru v0.5.3
	github.com/jaegertracing/jaeger v1.16.0
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/josephspurrier/goversioninfo v0.0.0-20200309025242-14b0ab84c6ca // indirect
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/klauspost/compress v1.9.3-0.20191122130757-c099ac9f21dd // indirect
	github.com/magefile/mage v1.9.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/modern-go/reflect2 v1.0.1
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191218182225-c300f1341702
	github.com/opentracing/opentracing-go v1.1.1-0.20190913142402-a7454ce5950e // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.1.3 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0 // indirect
	github.com/reviewdog/reviewdog v0.9.17
	github.com/ryanuber/go-glob v0.0.0-20170128012129-256dc444b735
	github.com/santhosh-tekuri/jsonschema v1.2.4
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/t-yuki/gocover-cobertura v0.0.0-20180217150009-aaee18c8195c
	github.com/ua-parser/uap-go v0.0.0-20200325213135-e1c09f13e2fe
	github.com/uber/tchannel-go v1.16.0 // indirect
	github.com/urso/magetools v0.0.0-20200125210132-c2e338f92f3a // indirect
	go.elastic.co/apm v1.8.0
	go.elastic.co/apm/module/apmelasticsearch v1.7.2
	go.elastic.co/apm/module/apmgrpc v1.7.0
	go.elastic.co/apm/module/apmhttp v1.7.2
	go.elastic.co/ecszap v0.2.0 // indirect
	go.elastic.co/fastjson v1.1.0 // indirect
	go.uber.org/atomic v1.6.0
	go.uber.org/zap v1.15.0
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	golang.org/x/tools v0.0.0-20200702044944-0cc1aa72b347 // indirect
	google.golang.org/grpc v1.29.1
	gopkg.in/yaml.v2 v2.3.0
	howett.net/plist v0.0.0-20200419221736-3b63eb3a43b5 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/Shopify/sarama => github.com/elastic/sarama v0.0.0-20191122160421-355d120d0970
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/docker/go-plugins-helpers => github.com/elastic/go-plugins-helpers v0.0.0-20200207104224-bdf17607b79f
	github.com/dop251/goja => github.com/andrewkroh/goja v0.0.0-20190128172624-dd2ac4456e20
	github.com/fsnotify/fsevents => github.com/elastic/fsevents v0.0.0-20181029231046-e1d381a4d270
	github.com/fsnotify/fsnotify => github.com/adriansr/fsnotify v0.0.0-20180417234312-c9bbe1f46f1d
	github.com/tonistiigi/fifo => github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c
	k8s.io/client-go => k8s.io/client-go v0.18.3
)
