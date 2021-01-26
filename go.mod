module github.com/elastic/apm-server

go 1.13

require (
	github.com/akavel/rsrc v0.10.1 // indirect
	github.com/apache/thrift v0.13.0
	github.com/cespare/xxhash/v2 v2.1.1
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/dgraph-io/badger/v2 v2.2007.3-0.20201012072640-f5a7e0a1c83b
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dop251/goja v0.0.0-20201212162034-be0895b77e07 // indirect
	github.com/dop251/goja_nodejs v0.0.0-20201201133918-0226646606a0 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/elastic/apm-server/approvaltest v0.0.0-00010101000000-000000000000
	github.com/elastic/beats/v7 v7.0.0-alpha2.0.20201215181550-5db4039630a6
	github.com/elastic/ecs v1.6.0
	github.com/elastic/gmux v0.1.0
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
	github.com/google/pprof v0.0.0-20201007051231-1066cbb265c7
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ianlancetaylor/demangle v0.0.0-20200715173712-053cf528c12f // indirect
	github.com/jaegertracing/jaeger v1.21.0
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/josephspurrier/goversioninfo v1.2.0 // indirect
	github.com/json-iterator/go v1.1.10
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/magefile/mage v1.10.0
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/modern-go/reflect2 v1.0.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0 // indirect
	github.com/reviewdog/reviewdog v0.9.17
	github.com/ryanuber/go-glob v0.0.0-20170128012129-256dc444b735
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/t-yuki/gocover-cobertura v0.0.0-20180217150009-aaee18c8195c
	github.com/urso/magetools v0.0.0-20200125210132-c2e338f92f3a // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20181112162635-ac52e6811b56
	go.elastic.co/apm v1.9.0
	go.elastic.co/apm/module/apmelasticsearch v1.7.2
	go.elastic.co/apm/module/apmgrpc v1.7.0
	go.elastic.co/apm/module/apmhttp v1.7.2
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/collector v0.17.0
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201208171446-5f87f3452ae9 // indirect
	golang.org/x/net v0.0.0-20201209123823-ac852fbbde11
	golang.org/x/sync v0.0.0-20201008141435-b3e1573b7520
	golang.org/x/sys v0.0.0-20201214210602-f9fddec55a1e // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20201215171152-6307297f4651
	google.golang.org/grpc v1.34.0
	gopkg.in/yaml.v2 v2.4.0
	howett.net/plist v0.0.0-20201203080718-1454fab16a06 // indirect
	k8s.io/client-go v12.0.0+incompatible // indirect
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
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/api/auditregistration/v1alpha1 => k8s.io/api/auditregistration/v1alpha1 v0.19.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
)
