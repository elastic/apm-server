module github.com/elastic/apm-server

go 1.21

toolchain go1.21.3

require (
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/dgraph-io/badger/v2 v2.2007.4
	github.com/dustin/go-humanize v1.0.1
	github.com/elastic/apm-aggregation v0.0.0-20230815024520-e75a37d9ddd6
	github.com/elastic/apm-data v1.0.0
	github.com/elastic/beats/v7 v7.17.16
	github.com/elastic/elastic-agent-client/v7 v7.8.0
	github.com/elastic/elastic-agent-libs v0.7.5
	github.com/elastic/elastic-agent-system-metrics v0.9.1
	github.com/elastic/gmux v0.3.1
	github.com/elastic/go-docappender v1.0.0
	github.com/elastic/go-elasticsearch/v8 v8.12.0
	github.com/elastic/go-sysinfo v1.12.0
	github.com/elastic/go-ucfg v0.8.6
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/gofrs/flock v0.8.1
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-cmp v0.6.0
	github.com/gorilla/mux v1.8.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v1.0.2
	github.com/jaegertracing/jaeger v1.53.0
	github.com/libp2p/go-reuseport v0.0.2
	github.com/modern-go/reflect2 v1.0.2
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.93.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	go.elastic.co/apm/module/apmelasticsearch/v2 v2.4.8
	go.elastic.co/apm/module/apmgorilla/v2 v2.4.8
	go.elastic.co/apm/module/apmgrpc/v2 v2.4.8
	go.elastic.co/apm/module/apmhttp/v2 v2.4.8
	go.elastic.co/apm/module/apmotel/v2 v2.4.8
	go.elastic.co/apm/v2 v2.4.8
	go.elastic.co/fastjson v1.3.0
	go.opentelemetry.io/collector/consumer v0.94.0
	go.opentelemetry.io/collector/pdata v1.1.0
	go.opentelemetry.io/otel v1.23.0
	go.opentelemetry.io/otel/metric v1.23.0
	go.opentelemetry.io/otel/sdk/metric v1.23.0
	go.uber.org/automaxprocs v1.5.3
	go.uber.org/zap v1.26.0
	golang.org/x/net v0.21.0
	golang.org/x/sync v0.6.0
	golang.org/x/term v0.17.0
	golang.org/x/time v0.5.0
	google.golang.org/grpc v1.61.0
	google.golang.org/protobuf v1.32.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/Shopify/sarama v1.38.1 // indirect
	github.com/apache/thrift v0.19.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/axiomhq/hyperloglog v0.0.0-20230201085229-3ddf4bad03dc // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cockroachdb/errors v1.8.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20190617123548-eb05cc24525f // indirect
	github.com/cockroachdb/pebble v0.0.0-20230627193317-c807f60529a3 // indirect
	github.com/cockroachdb/redact v1.0.8 // indirect
	github.com/cockroachdb/sentry-go v0.6.1-cockroachdb.2 // indirect
	github.com/cockroachdb/tokenbucket v0.0.0-20230613231145-182959a1fad6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/dlclark/regexp2 v1.8.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dop251/goja v0.0.0-20230427124612-428fc442ff5f // indirect
	github.com/dop251/goja_nodejs v0.0.0-20230322100729-2550c7b6c124 // indirect
	github.com/eapache/go-resiliency v1.4.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/elastic/ecs v1.12.0 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.4.0 // indirect
	github.com/elastic/go-lumber v0.1.2-0.20220819171948-335fde24ea0f // indirect
	github.com/elastic/go-structform v0.0.10 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/elastic/gosigar v0.14.2 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v1.8.9 // indirect
	github.com/h2non/filetype v1.1.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/joeshaw/multierror v0.0.0-20140124173710-69b34d4ec901 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.93.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/xdg/scram v1.0.5 // indirect
	github.com/xdg/stringprep v1.0.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.elastic.co/apm v1.11.0 // indirect
	go.elastic.co/apm/module/apmelasticsearch v1.7.2 // indirect
	go.elastic.co/apm/module/apmhttp v1.7.2 // indirect
	go.elastic.co/apm/module/apmzap/v2 v2.4.8 // indirect
	go.elastic.co/ecszap v1.0.2 // indirect
	go.opentelemetry.io/collector/semconv v0.93.0 // indirect
	go.opentelemetry.io/otel/sdk v1.23.0 // indirect
	go.opentelemetry.io/otel/trace v1.23.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.19.0 // indirect
	golang.org/x/exp v0.0.0-20231127185646-65229373498e // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240102182953-50ed04b92917 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.5.0 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	howett.net/plist v1.0.0 // indirect
)

replace (
	github.com/Shopify/sarama => github.com/elastic/sarama v1.19.1-0.20210823122811-11c3ef800752
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/docker/go-plugins-helpers => github.com/elastic/go-plugins-helpers v0.0.0-20200207104224-bdf17607b79f
	github.com/dop251/goja => github.com/andrewkroh/goja v0.0.0-20190128172624-dd2ac4456e20
	github.com/dop251/goja_nodejs => github.com/dop251/goja_nodejs v0.0.0-20171011081505-adff31b136e6 // pin to version used by beats
	// We replace golang/glog, which is used by ristretto, to avoid polluting the
	// command line flags and conflicting with command line flags added by libbeat.
	github.com/golang/glog => ./internal/glog
)
