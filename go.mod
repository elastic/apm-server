module github.com/elastic/apm-server

go 1.23.0

require (
	github.com/KimMachineGun/automemlimit v0.7.0-pre.3
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/cockroachdb/pebble/v2 v2.0.2
	github.com/dustin/go-humanize v1.0.1
	github.com/elastic/apm-aggregation v1.2.0
	github.com/elastic/apm-data v1.15.0
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20241231140711-7806f1a2cb26
	github.com/elastic/elastic-agent-client/v7 v7.17.0
	github.com/elastic/elastic-agent-libs v0.18.0
	github.com/elastic/elastic-agent-system-metrics v0.11.7
	github.com/elastic/gmux v0.3.2
	github.com/elastic/go-docappender/v2 v2.3.3
	github.com/elastic/go-elasticsearch/v8 v8.17.0
	github.com/elastic/go-sysinfo v1.15.0
	github.com/elastic/go-ucfg v0.8.8
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible
	github.com/gofrs/flock v0.12.1
	github.com/gofrs/uuid/v5 v5.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-cmp v0.6.0
	github.com/gorilla/mux v1.8.1
	github.com/hashicorp/golang-lru v1.0.2
	github.com/joeshaw/multierror v0.0.0-20140124173710-69b34d4ec901
	github.com/libp2p/go-reuseport v0.4.0
	github.com/modern-go/reflect2 v1.0.2
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.10.0
	go.elastic.co/apm/module/apmelasticsearch/v2 v2.6.2
	go.elastic.co/apm/module/apmgorilla/v2 v2.6.2
	go.elastic.co/apm/module/apmgrpc/v2 v2.6.2
	go.elastic.co/apm/module/apmhttp/v2 v2.6.2
	go.elastic.co/apm/module/apmotel/v2 v2.6.2
	go.elastic.co/apm/v2 v2.6.2
	go.elastic.co/fastjson v1.4.0
	go.opentelemetry.io/collector/pdata v1.22.0
	go.opentelemetry.io/otel v1.33.0
	go.opentelemetry.io/otel/metric v1.33.0
	go.opentelemetry.io/otel/sdk/metric v1.33.0
	go.uber.org/automaxprocs v1.6.0
	go.uber.org/zap v1.27.0
	go.uber.org/zap/exp v0.3.0
	golang.org/x/net v0.34.0
	golang.org/x/sync v0.10.0
	golang.org/x/term v0.28.0
	golang.org/x/time v0.9.0
	google.golang.org/grpc v1.69.2
	google.golang.org/protobuf v1.36.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/zstd v1.5.6 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/axiomhq/hyperloglog v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cockroachdb/crlib v0.0.0-20241015224233-894974b3ad94 // indirect
	github.com/cockroachdb/errors v1.11.3 // indirect
	github.com/cockroachdb/fifo v0.0.0-20240816210425-c5d0cb0b6fc0 // indirect
	github.com/cockroachdb/logtags v0.0.0-20230118201751-21c54148d20b // indirect
	github.com/cockroachdb/pebble v1.1.2 // indirect
	github.com/cockroachdb/redact v1.1.5 // indirect
	github.com/cockroachdb/swiss v0.0.0-20240612210725-f4de07ae6964 // indirect
	github.com/cockroachdb/tokenbucket v0.0.0-20230807174530-cc333fc44b06 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-metro v0.0.0-20211217172704-adc40b04c140 // indirect
	github.com/dlclark/regexp2 v1.8.1 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/dop251/goja v0.0.0-20230427124612-428fc442ff5f // indirect
	github.com/dop251/goja_nodejs v0.0.0-20171011081505-adff31b136e6 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.6.0 // indirect
	github.com/elastic/go-lumber v0.1.2-0.20220819171948-335fde24ea0f // indirect
	github.com/elastic/go-structform v0.0.10 // indirect
	github.com/elastic/go-windows v1.0.2 // indirect
	github.com/elastic/gosigar v0.14.3 // indirect
	github.com/elastic/opentelemetry-lib v0.14.0 // indirect
	github.com/elastic/pkcs8 v1.0.0 // indirect
	github.com/elastic/sarama v1.19.1-0.20241120141909-c7eabfcee7e5 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/getsentry/sentry-go v0.29.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.5-0.20231225225746-43d5d4cd4e0e // indirect
	github.com/gomodule/redigo v1.8.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/h2non/filetype v1.1.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.60.1 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/shirou/gopsutil/v4 v4.24.9 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.elastic.co/apm/module/apmzap/v2 v2.6.2 // indirect
	go.elastic.co/ecszap v1.0.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer v1.22.0 // indirect
	go.opentelemetry.io/collector/semconv v0.116.0 // indirect
	go.opentelemetry.io/otel/sdk v1.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.33.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/exp v0.0.0-20241009180824-f66d83c29e7c // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241209162323-e6fa225c2576 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	howett.net/plist v1.0.1 // indirect
)

replace github.com/dop251/goja => github.com/elastic/goja v0.0.0-20190128172624-dd2ac4456e20 // pin to version used by beats
