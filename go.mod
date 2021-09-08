module github.com/elastic/apm-server

go 1.13

require (
	github.com/DataDog/zstd v1.4.4 // indirect
	github.com/apache/thrift v0.14.2
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/dgraph-io/badger/v2 v2.2007.3-0.20201012072640-f5a7e0a1c83b
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dop251/goja v0.0.0-20210904102640-6338b3246846 // indirect
	github.com/dop251/goja_nodejs v0.0.0-20210225215109-d91c329300e7 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/elastic/apm-server/approvaltest v0.0.0-00010101000000-000000000000
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20210908114441-c6b31a439205
	github.com/elastic/ecs v1.11.0
	github.com/elastic/gmux v0.1.0
	github.com/elastic/go-elasticsearch/v7 v7.5.1-0.20210728153421-6462d8b84e7d
	github.com/elastic/go-elasticsearch/v8 v8.0.0-20210727161915-8cf93274b968
	github.com/elastic/go-hdrhistogram v0.1.0
	github.com/elastic/go-ucfg v0.8.4-0.20200415140258-1232bd4774a6
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/gogo/protobuf v1.3.2
	github.com/google/pprof v0.0.0-20210609004039-a478d1d731e9
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jaegertracing/jaeger v1.25.0
	github.com/josephspurrier/goversioninfo v1.3.0 // indirect
	github.com/json-iterator/go v1.1.11
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/libp2p/go-reuseport v0.0.2
	github.com/magefile/mage v1.11.0
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/modern-go/reflect2 v1.0.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.34.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/poy/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.6.5
	github.com/tidwall/sjson v1.1.1
	github.com/urso/magetools v0.0.0-20200125210132-c2e338f92f3a // indirect
	github.com/xeipuuv/gojsonschema v1.2.0
	go.elastic.co/apm v1.13.1
	go.elastic.co/apm/module/apmelasticsearch v1.12.0
	go.elastic.co/apm/module/apmgrpc v1.12.0
	go.elastic.co/apm/module/apmhttp v1.12.0
	go.elastic.co/ecszap v1.0.0 // indirect
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/collector v0.34.0
	go.opentelemetry.io/collector/model v0.34.0
	go.uber.org/atomic v1.9.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/mod v0.5.0 // indirect
	golang.org/x/net v0.0.0-20210907225631-ff17edfbf26d
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210906170528-6f6e22806c34 // indirect
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	golang.org/x/tools v0.1.5
	google.golang.org/genproto v0.0.0-20210903162649-d08c68adba83 // indirect
	google.golang.org/grpc v1.40.0
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools/gotestsum v1.7.0
	howett.net/plist v0.0.0-20201203080718-1454fab16a06 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/Microsoft/go-winio => github.com/bi-zone/go-winio v0.4.15
	github.com/Shopify/sarama => github.com/elastic/sarama v1.19.1-0.20210120173147-5c8cb347d877
	github.com/aws/aws-sdk-go-v2 => github.com/aws/aws-sdk-go-v2 v0.9.0
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/docker/go-plugins-helpers => github.com/elastic/go-plugins-helpers v0.0.0-20200207104224-bdf17607b79f
	github.com/dop251/goja => github.com/andrewkroh/goja v0.0.0-20190128172624-dd2ac4456e20
	github.com/dop251/goja_nodejs => github.com/dop251/goja_nodejs v0.0.0-20171011081505-adff31b136e6 // pin to version used by beats
	github.com/elastic/apm-server/approvaltest => ./approvaltest
	github.com/fsnotify/fsevents => github.com/elastic/fsevents v0.0.0-20181029231046-e1d381a4d270
	github.com/fsnotify/fsnotify => github.com/adriansr/fsnotify v0.0.0-20180417234312-c9bbe1f46f1d
	github.com/tonistiigi/fifo => github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c
	golang.org/x/tools => golang.org/x/tools v0.1.2
)

// We replace golang/glog, which is used by ristretto, to avoid polluting the
// command line flags and conflicting with command line flags added by libbeat.
replace github.com/golang/glog => ./internal/glog

replace go.opentelemetry.io/collector => ./internal/otel_collector
