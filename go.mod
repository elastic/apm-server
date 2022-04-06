module github.com/elastic/apm-server

go 1.17

require (
	github.com/cespare/xxhash/v2 v2.1.2
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/dgraph-io/badger/v2 v2.2007.3-0.20201012072640-f5a7e0a1c83b
	github.com/dustin/go-humanize v1.0.0
	github.com/elastic/apm-server/approvaltest v0.0.0-00010101000000-000000000000
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20220406030009-6c32e78dcbfa
	github.com/elastic/ecs v1.12.0
	github.com/elastic/gmux v0.2.0
	github.com/elastic/go-elasticsearch/v8 v8.1.0
	github.com/elastic/go-hdrhistogram v0.1.0
	github.com/elastic/go-ucfg v0.8.4
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/gogo/protobuf v1.3.2
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jaegertracing/jaeger v1.30.0
	github.com/json-iterator/go v1.1.12
	github.com/libp2p/go-reuseport v0.0.2
	github.com/magefile/mage v1.13.0
	github.com/modern-go/reflect2 v1.0.2
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.44.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.elastic.co/apm v1.15.0
	go.elastic.co/apm/module/apmelasticsearch v1.15.0
	go.elastic.co/apm/module/apmgorilla v1.15.0
	go.elastic.co/apm/module/apmgrpc v1.15.0
	go.elastic.co/apm/module/apmhttp v1.15.0
	go.elastic.co/apm/module/apmzap v1.15.0
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/collector v0.44.0
	go.opentelemetry.io/collector/model v0.44.0
	go.uber.org/zap v1.21.0
	golang.org/x/net v0.0.0-20220403103023-749bd193bc2b
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	golang.org/x/tools v0.1.10
	google.golang.org/grpc v1.45.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gotest.tools/gotestsum v1.7.0
)

require (
	github.com/DataDog/zstd v1.4.4 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Shopify/sarama v1.30.1 // indirect
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/apache/thrift v0.15.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/containerd/containerd v1.6.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dnephin/pflag v1.0.7 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.7+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dop251/goja v0.0.0-20220405120441-9037c2b61cbf // indirect
	github.com/dop251/goja_nodejs v0.0.0-20211022123610-8dd9abb0616d // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/elastic/elastic-agent-client/v7 v7.0.0-20210922110810-e6f1f402a9ed // indirect
	github.com/elastic/elastic-transport-go/v8 v8.1.0 // indirect
	github.com/elastic/go-concert v0.2.0 // indirect
	github.com/elastic/go-licenser v0.4.0 // indirect
	github.com/elastic/go-lumber v0.1.0 // indirect
	github.com/elastic/go-seccomp-bpf v1.2.0 // indirect
	github.com/elastic/go-structform v0.0.9 // indirect
	github.com/elastic/go-sysinfo v1.7.1 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/elastic/gosigar v0.14.2 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v1.8.3 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/h2non/filetype v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/ianlancetaylor/demangle v0.0.0-20200824232613-28f6c0f3b639 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jcchavezs/porto v0.4.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.2 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/joeshaw/multierror v0.0.0-20140124173710-69b34d4ec901 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/josephspurrier/goversioninfo v1.4.0 // indirect
	github.com/klauspost/compress v1.14.2 // indirect
	github.com/knadh/koanf v1.4.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/miekg/dns v1.1.42 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/mostynb/go-grpc-compression v1.1.16 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.44.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rs/cors v1.8.2 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4 // indirect
	github.com/shirou/gopsutil/v3 v3.21.12 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/tidwall/gjson v1.9.3 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/sjson v1.1.1 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/urso/diag v0.0.0-20200210123136-21b3cc8eb797 // indirect
	github.com/urso/sderr v0.0.0-20210525210834-52b04e8f5c71 // indirect
	github.com/xdg/scram v1.0.3 // indirect
	github.com/xdg/stringprep v1.0.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.elastic.co/ecszap v1.0.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.28.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.28.0 // indirect
	go.opentelemetry.io/otel v1.3.0 // indirect
	go.opentelemetry.io/otel/internal/metric v0.26.0 // indirect
	go.opentelemetry.io/otel/metric v0.26.0 // indirect
	go.opentelemetry.io/otel/trace v1.3.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/crypto v0.0.0-20220331220935-ae2d96664a29 // indirect
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/sys v0.0.0-20220405210540-1e041c57c461 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220405205423-9d709892a2bf // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.5.0 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	howett.net/plist v1.0.0 // indirect
	k8s.io/api v0.23.0 // indirect
	k8s.io/apimachinery v0.23.0 // indirect
	k8s.io/client-go v0.23.0 // indirect
	k8s.io/klog/v2 v2.30.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/Microsoft/go-winio => github.com/bi-zone/go-winio v0.4.15
	github.com/Shopify/sarama => github.com/elastic/sarama v1.19.1-0.20210823122811-11c3ef800752
	github.com/aws/aws-sdk-go-v2 => github.com/aws/aws-sdk-go-v2 v0.9.0
	github.com/containerd/containerd => github.com/containerd/containerd v1.6.1
	github.com/docker/docker => github.com/docker/engine v0.0.0-20191113042239-ea84732a7725
	github.com/docker/go-plugins-helpers => github.com/elastic/go-plugins-helpers v0.0.0-20200207104224-bdf17607b79f
	github.com/dop251/goja => github.com/andrewkroh/goja v0.0.0-20190128172624-dd2ac4456e20
	github.com/dop251/goja_nodejs => github.com/dop251/goja_nodejs v0.0.0-20171011081505-adff31b136e6 // pin to version used by beats
	github.com/elastic/apm-server/approvaltest => ./approvaltest
	github.com/fsnotify/fsevents => github.com/elastic/fsevents v0.0.0-20181029231046-e1d381a4d270
	github.com/fsnotify/fsnotify => github.com/adriansr/fsnotify v1.4.8-0.20211018144411-a81f2b630e7c
	// We replace golang/glog, which is used by ristretto, to avoid polluting the
	// command line flags and conflicting with command line flags added by libbeat.
	github.com/golang/glog => ./internal/glog
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2
	github.com/tonistiigi/fifo => github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c
	golang.org/x/tools => golang.org/x/tools v0.1.2
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

replace go.opentelemetry.io/collector => ./internal/otel_collector
