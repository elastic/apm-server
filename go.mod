module github.com/elastic/apm-server

go 1.13

require (
	github.com/akavel/rsrc v0.9.0 // indirect
	github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/client9/misspell v0.3.5-0.20180309020325-c0b55c823952 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.3-0.20201012072640-f5a7e0a1c83b
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dop251/goja v0.0.0-20201107160812-7545ac6de48a // indirect
	github.com/dop251/goja_nodejs v0.0.0-20200811150831-9bc458b4bbeb // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/elastic/apm-server/approvaltest v0.0.0-00010101000000-000000000000
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20201112141016-0fda3061815d
	github.com/elastic/ecs v1.6.0
	github.com/elastic/go-elasticsearch/v7 v7.5.1-0.20201007132508-ff965d99ba02
	github.com/elastic/go-elasticsearch/v8 v8.0.0-20201007143536-4b4020669208
	github.com/elastic/go-hdrhistogram v0.1.0
	github.com/elastic/go-licenser v0.3.1
	github.com/elastic/go-sysinfo v1.4.0 // indirect
	github.com/elastic/go-ucfg v0.8.4-0.20200415140258-1232bd4774a6
	github.com/fatih/color v1.10.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/gofrs/uuid v3.3.0+incompatible
	github.com/gogo/googleapis v1.3.1-0.20190914144012-b8d18e97a9a1 // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4 // indirect
	github.com/google/pprof v0.0.0-20200708004538-1a94d8640e99
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/ianlancetaylor/demangle v0.0.0-20200715173712-053cf528c12f // indirect
	github.com/jaegertracing/jaeger v1.16.0
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/josephspurrier/goversioninfo v1.2.0 // indirect
	github.com/json-iterator/go v1.1.10
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/klauspost/compress v1.9.3-0.20191122130757-c099ac9f21dd // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/magefile/mage v1.10.0
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/modern-go/reflect2 v1.0.1
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191218182225-c300f1341702
	github.com/opentracing/opentracing-go v1.1.1-0.20190913142402-a7454ce5950e // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0 // indirect
	github.com/reviewdog/reviewdog v0.9.17
	github.com/ryanuber/go-glob v0.0.0-20170128012129-256dc444b735
	github.com/santhosh-tekuri/jsonschema v1.2.4
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/t-yuki/gocover-cobertura v0.0.0-20180217150009-aaee18c8195c
	github.com/uber/tchannel-go v1.16.0 // indirect
	github.com/urso/magetools v0.0.0-20200125210132-c2e338f92f3a // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20181112162635-ac52e6811b56
	go.elastic.co/apm v1.9.0
	go.elastic.co/apm/module/apmelasticsearch v1.7.2
	go.elastic.co/apm/module/apmgrpc v1.7.0
	go.elastic.co/apm/module/apmhttp v1.7.2
	go.elastic.co/fastjson v1.1.0
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/sys v0.0.0-20201112073958-5cba982894dd // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	golang.org/x/tools v0.0.0-20201111224557-41a3a589386c
	google.golang.org/grpc v1.29.1
	gopkg.in/yaml.v2 v2.3.0
	howett.net/plist v0.0.0-20201026045517-117a925f2150 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/Shopify/sarama => github.com/elastic/sarama v0.0.0-20191122160421-355d120d0970
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/docker/go-plugins-helpers => github.com/elastic/go-plugins-helpers v0.0.0-20200207104224-bdf17607b79f
	github.com/dop251/goja => github.com/andrewkroh/goja v0.0.0-20190128172624-dd2ac4456e20
	github.com/dop251/goja_nodejs => github.com/dop251/goja_nodejs v0.0.0-20171011081505-adff31b136e6 // pin to version used by beats
	github.com/elastic/apm-server/approvaltest => ./approvaltest
	github.com/fsnotify/fsevents => github.com/elastic/fsevents v0.0.0-20181029231046-e1d381a4d270
	github.com/fsnotify/fsnotify => github.com/adriansr/fsnotify v0.0.0-20180417234312-c9bbe1f46f1d
	github.com/tonistiigi/fifo => github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c
	golang.org/x/tools => golang.org/x/tools v0.0.0-20200602230032-c00d67ef29d0 // release 1.14
	k8s.io/client-go => k8s.io/client-go v0.18.3
)

replace github.com/dgraph-io/badger/v2 => github.com/elastic/badger/v2 v2.2007.2-beats
