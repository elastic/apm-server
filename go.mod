module github.com/elastic/apm-server

go 1.19

require (
	github.com/axiomhq/hyperloglog v0.0.0-20220105174342-98591331716a
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/dgraph-io/badger/v2 v2.2007.3-0.20201012072640-f5a7e0a1c83b
	github.com/dustin/go-humanize v1.0.0
	github.com/elastic/apm-data v0.1.1-0.20230309014206-3ad1a5caedc9
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20230411141035-34614bc17375
	github.com/elastic/elastic-agent-client/v7 v7.1.0
	github.com/elastic/elastic-agent-libs v0.3.3
	github.com/elastic/elastic-agent-system-metrics v0.6.0
	github.com/elastic/gmux v0.2.0
	github.com/elastic/go-docappender v0.1.0
	github.com/elastic/go-elasticsearch/v8 v8.7.0
	github.com/elastic/go-hdrhistogram v0.1.0
	github.com/elastic/go-sysinfo v1.10.1
	github.com/elastic/go-ucfg v0.8.6
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/goccy/go-json v0.10.2
	github.com/gofrs/flock v0.8.1
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.9
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hnlq715/golang-lru v0.3.1
	github.com/jaegertracing/jaeger v1.38.1
	github.com/json-iterator/go v1.1.12
	github.com/libp2p/go-reuseport v0.0.2
	github.com/modern-go/reflect2 v1.0.2
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.63.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.1
	github.com/tidwall/gjson v1.14.2
	go.elastic.co/apm/module/apmelasticsearch/v2 v2.2.0
	go.elastic.co/apm/module/apmgorilla/v2 v2.2.0
	go.elastic.co/apm/module/apmgrpc/v2 v2.2.0
	go.elastic.co/apm/module/apmhttp/v2 v2.2.0
	go.elastic.co/apm/v2 v2.2.0
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/collector v0.63.1
	go.opentelemetry.io/collector/pdata v0.63.1
	go.uber.org/automaxprocs v1.5.1
	go.uber.org/zap v1.24.0
	golang.org/x/net v0.8.0
	golang.org/x/sync v0.1.0
	golang.org/x/term v0.6.0
	golang.org/x/time v0.3.0
	google.golang.org/grpc v1.53.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/zstd v1.4.4 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/Shopify/sarama v1.32.0 // indirect
	github.com/apache/thrift v0.17.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/containerd/containerd v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/dlclark/regexp2 v1.8.1 // indirect
	github.com/dop251/goja v0.0.0-20230304130813-e2f543bf4b4c // indirect
	github.com/dop251/goja_nodejs v0.0.0-20230226152057-060fa99b809f // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/elastic/elastic-agent-shipper-client v0.5.1-0.20230228231646-f04347b666f3 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.2.0 // indirect
	github.com/elastic/go-licenser v0.4.1 // indirect
	github.com/elastic/go-lumber v0.1.2-0.20220819171948-335fde24ea0f // indirect
	github.com/elastic/go-structform v0.0.10 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/elastic/gosigar v0.14.2 // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/frankban/quicktest v1.14.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v1.8.3 // indirect
	github.com/h2non/filetype v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jcchavezs/porto v0.4.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.2 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/joeshaw/multierror v0.0.0-20140124173710-69b34d4ec901 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magefile/mage v1.14.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.63.0 // indirect
	github.com/opencontainers/image-spec v1.0.3-0.20211202183452-c5a74bcca799 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/shirou/gopsutil/v3 v3.22.9 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/xdg/scram v1.0.3 // indirect
	github.com/xdg/stringprep v1.0.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.elastic.co/apm/module/apmzap/v2 v2.2.0 // indirect
	go.elastic.co/ecszap v1.0.1 // indirect
	go.opentelemetry.io/collector/semconv v0.63.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20230306155012-7f2fa6fef1f4 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.5.0 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	howett.net/plist v1.0.0 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/Microsoft/go-winio => github.com/bi-zone/go-winio v0.4.15
	github.com/Shopify/sarama => github.com/elastic/sarama v1.19.1-0.20210823122811-11c3ef800752
	github.com/aws/aws-sdk-go-v2 => github.com/aws/aws-sdk-go-v2 v0.9.0
	github.com/containerd/containerd => github.com/containerd/containerd v1.6.6
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/docker/go-plugins-helpers => github.com/elastic/go-plugins-helpers v0.0.0-20200207104224-bdf17607b79f
	github.com/dop251/goja => github.com/andrewkroh/goja v0.0.0-20190128172624-dd2ac4456e20
	github.com/dop251/goja_nodejs => github.com/dop251/goja_nodejs v0.0.0-20171011081505-adff31b136e6 // pin to version used by beats
	github.com/fsnotify/fsevents => github.com/elastic/fsevents v0.0.0-20181029231046-e1d381a4d270
	github.com/fsnotify/fsnotify => github.com/adriansr/fsnotify v1.4.8-0.20211018144411-a81f2b630e7c
	// We replace golang/glog, which is used by ristretto, to avoid polluting the
	// command line flags and conflicting with command line flags added by libbeat.
	github.com/golang/glog => ./internal/glog
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2
	github.com/tonistiigi/fifo => github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c
)

// Exclude old modules (with security vulnerabilities) used only by tests of dependencies.
exclude (
	github.com/buger/jsonparser v0.0.0-20180808090653-f4dd9f5a6b44
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dgrijalva/jwt-go/v4 v4.0.0-preview1
	github.com/gin-gonic/gin v1.5.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.0.0-20190115041553-12f6a991201f
	github.com/opencontainers/runc v1.0.0-rc9
	github.com/opencontainers/runc v1.0.2
	go.mongodb.org/mongo-driver v1.0.3
	go.mongodb.org/mongo-driver v1.1.1
	go.mongodb.org/mongo-driver v1.3.0
	go.mongodb.org/mongo-driver v1.3.4
	go.mongodb.org/mongo-driver v1.4.3
	go.mongodb.org/mongo-driver v1.4.4
	go.mongodb.org/mongo-driver v1.4.6
)

exclude github.com/elastic/elastic-agent v0.0.0-20220831162706-5f1e54f40d3e

// Workaround until https://github.com/axiomhq/hyperloglog/pull/33 PR is released
exclude github.com/influxdata/influxdb v1.7.6
