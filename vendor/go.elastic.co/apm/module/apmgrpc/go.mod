module go.elastic.co/apm/module/apmgrpc

require (
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/stretchr/testify v1.4.0
	go.elastic.co/apm v1.7.0
	go.elastic.co/apm/module/apmhttp v1.7.0
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80
	google.golang.org/grpc v1.17.0
)

replace go.elastic.co/apm => ../..

replace go.elastic.co/apm/module/apmhttp => ../apmhttp

go 1.13
