[![GoDoc](https://godoc.org/github.com/elastic/apm-agent-go?status.svg)](http://godoc.org/github.com/elastic/apm-agent-go)
[![Travis-CI](https://travis-ci.org/elastic/apm-agent-go.svg)](https://travis-ci.org/elastic/apm-agent-go)
[![AppVeyor](https://ci.appveyor.com/api/projects/status/28fhswvqqc7p90f7?svg=true)](https://ci.appveyor.com/project/AndrewWilkins/apm-agent-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/elastic/apm-agent-go)](https://goreportcard.com/report/github.com/elastic/apm-agent-go)
[![codecov.io](https://codecov.io/github/elastic/apm-agent-go/coverage.svg?branch=master)](https://codecov.io/github/elastic/apm-agent-go?branch=master)

# apm-agent-go: APM Agent for Go (pre-alpha)

This is the official Go package for [Elastic APM][].

The Go agent enables you to trace the execution of operations in your application,
sending performance metrics and errors to the Elastic APM server.

This repository is in a pre-alpha state and under heavy development.
Do not deploy into production!

## Installation

```bash
go get -u github.com/elastic/apm-agent-go
```

## Requirements

Tested with Go 1.8+.

## License

Apache 2.0.

## Getting Help

If you find a bug, please [report an issue](https://github.com/elastic/apm-agent-go/issues).
For any other assistance, please open or add to a topic on the [APM discuss forum](https://discuss.elastic.co/c/apm).

If you need help or hit an issue, please start by opening a topic on our discuss forums.
Please note that we reserve GitHub tickets for confirmed bugs and enhancement requests.

## Configuration

The Go agent can be configured in code, or via environment variables.

The two most critical configuration attributes are the server URL and
the secret token. All other attributes have default values, and are not
required to enable tracing.

Environment variable                    | Default | Description
----------------------------------------|---------|------------------------------------------
ELASTIC\_APM\_SERVER\_URL               |         | Base URL of the Elastic APM server. If unspecified, no tracing will take place.
ELASTIC\_APM\_SECRET\_TOKEN             |         | The secret token for Elastic APM server.
ELASTIC\_APM\_VERIFY\_SERVER\_CERT      | true    | Verify certificates when using https.
ELASTIC\_APM\_FLUSH\_INTERVAL           | 10s     | Time to wait before sending transactions to the Elastic APM server. Transactions will be batched up and sent periodically.
ELASTIC\_APM\_MAX\_QUEUE\_SIZE          | 500     | Maximum number of transactions to queue before sending to the Elastic APM server. Once this number is reached, any new transactions will replace old ones until the queue is flushed.
ELASTIC\_APM\_TRANSACTION\_MAX\_SPANS   | 500     | Maximum number of spans to capture per transaction. After this is reached, new spans will not be created, and a dropped count will be incremented.
ELASTIC\_APM\_TRANSACTION\_SAMPLE\_RATE | 1.0     | Number in the range 0.0-1.0 inclusive, controlling how many transactions should be sampled (i.e. include full detail.)
ELASTIC\_APM\_ENVIRONMENT               |         | Environment name, e.g. "production".
ELASTIC\_APM\_FRAMEWORK\_NAME           |         | Framework name, e.g. "gin".
ELASTIC\_APM\_FRAMEWORK\_VERSION        |         | Framework version, e.g. "1.0".
ELASTIC\_APM\_SERVICE\_NAME             |         | Service name, e.g. "my-service". If this is unspecified, the agent will report the program binary name as the service name.
ELASTIC\_APM\_SERVICE\_VERSION          |         | Service version, e.g. "1.0".
ELASTIC\_APM\_HOSTNAME                  |         | Override for the hostname.

## Instrumentation

The Go agent includes instrumentation for various standard packages, including
but not limited to `net/http` and `database/sql`. You can find an overview of
the included instrumentation modules below, as well as an overview of how to add
custom instrumentation to your application.

## net/http

Package `contrib/apmhttp` can be used to wrap `net/http` handlers:

```go
import (
	"github.com/elastic/apm-agent-go/contrib/apmhttp"
)

func main() {
	var myHandler http.Handler = ...
	tracedHandler := apmhttp.Handler{
		Handler: myHandler,
	}
}
```

If you want your handler to recover panics and send them to Elastic APM,
then you can set the Recovery field of apmhttp.Handler:

```go
apmhttp.Handler{
	Handler: myHandler,
	Recovery: apmhttp.NewTraceHandler(nil),
}
```

### Gin

Package `contrib/apmgin` provides middleware for [Gin](https://github.com/gin-gonic/gin):

```go
import (
	"github.com/elastic/apm-agent-go/contrib/apmgin"
)

func main() {
	engine := gin.New()
	engine.Use(apmgin.Middleware(engine, nil))
	...
}
```

The apmgin middleware will recover panics and send them to Elastic APM,
so you do not need to install the gin.Recovery middleware.

### gorilla/mux

Package `contrib/apmgorilla` provides middleware for [gorilla/mux](https://github.com/gorilla/mux):

```go
import (
	"github.com/gorilla/mux"

	"github.com/elastic/apm-agent-go/contrib/apmgorilla"
)

func main() {
	router := mux.NewRouter()
	router.Use(apmgorilla.Middleware(nil)) // nil for default tracer
	...
}
```

The apmgorilla middleware will recover panics and send them to Elastic APM,
so you do not need to install any other recovery middleware.

### httprouter

Package `contrib/apmhttprouter` provides a handler wrapper for [httprouter](https://github.com/julienschmidt/httprouter):

```go
import (
	"github.com/julienschmidt/httprouter"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmhttp"
	"github.com/elastic/apm-agent-go/contrib/apmhttprouter"
)

func main() {
	router := httprouter.New()

	const route = "/my/route"
	tracer := elasticapm.DefaultTracer
	recovery := apmhttp.NewTraceRecovery(tracer) // optional panic recovery
	router.GET(route, apmhttprouter.WrapHandle(h, tracer, route, recovery))
	...
}
```

httprouter [does not provide a means of obtaining the matched route](https://github.com/julienschmidt/httprouter/pull/139),
hence the route must be passed into the wrapper.

### Echo

Package `contrib/apmecho` provides middleware for [Echo](https://github.com/labstack/echo):

```go
import (
	"github.com/labstack/echo"
	"github.com/elastic/apm-agent-go/contrib/apmecho"
)

func main() {
	e := echo.New()
	e.Use(apmecho.Middleware(nil)) // nil for default tracer
	...
}
```

The apmecho middleware will recover panics and send them to Elastic APM,
so you do not need to install the echo/middleware.Recover middleware.

### AWS Lambda

Package `contrib/apmlambda` intercepts and reports transactions for [AWS Lambda Go](https://github.com/aws/aws-lambda-go)
functions. Importing the package is enough to report the function invocations.

```go
import (
	_ "github.com/elastic/apm-agent-go/contrib/apmlambda"
)
```

We currently do not expose the transactions via `context`; when we do, it will
be necessary to make a small change to your code to call apmlambda.Start instead
of lambda.Start.

### database/sql

Package `contrib/apmsql` provides methods for wrapping `database/sql/driver.Drivers`,
tracing queries as spans. To trace SQL queries, you should register drivers using
`apmsql.Register` and obtain connections with `apmsql.Open`. The parameters are
exactly the same as if you were to call `sql.Register` and `sql.Open` respectively.

As a convenience, we also provide packages which will automatically register popular
drivers with `apmsql.Register`: `contrib/apmsql/pq` and `contrib/apmsql/sqlite3`. e.g.

```go
import (
	"github.com/elastic/apm-agent-go/contrib/apmsql"
	_ "github.com/elastic/apm-agent-go/contrib/apmsql/pq"
	_ "github.com/elastic/apm-agent-go/contrib/apmsql/sqlite3"
)

func main() {
	db, err := apmsql.Open("pq", "postgres://...")
	db, err := apmsql.Open("sqlite3", ":memory:")
}
```

Spans will be created for queries and other statement executions if the context
methods are used, and the context includes a transaction.

### gRPC

Package `contrib/apmgrpc` provides unary interceptors for [gprc-go](https://github.com/grpc/grpc-go),
for tracing incoming server requests as transactions, and outgoing client
requests as spans:

```go
import (
	"github.com/elastic/apm-agent-go/contrib/apmgrpc"
)

func main() {
	server := grpc.NewServer(grpc.UnaryInterceptor(apmgrpc.NewUnaryServerInterceptor()))
	...
	conn, err := grpc.Dial(addr, grpc.WithUnaryInterceptor(apmgrpc.NewUnaryClientInterceptor()))
	...
}
```

The server interceptor can optionally be made to recover panics, in the
same way as [grpc\_recovery](https://github.com/grpc-ecosystem/go-grpc-middleware/tree/master/recovery).
The apmgrpc server interceptor will always send panics it observes as
errors to the Elastic APM server. If you want to recover panics but also
want to continue using grpc\_recovery, then you should ensure that it
comes before the apmgrpc interceptor in the interceptor chain, or panics
will not be captured by apmgrpc.

```go
server := grpc.NewServer(grpc.UnaryInterceptor(
	apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery()),
))
...
```

There is currently no support for intercepting at the stream level. Please
file an issue and/or send a pull request if this is something you need.

### Custom instrumentation

For custom instrumentation, [elasticapm.Tracer](https://godoc.org/github.com/elastic/apm-agent-go#Tracer)
provides the entry point to instrumenting your application. You can use the
`elasticapm.DefaultTracer`, which is configured based on `ELASTIC_APM_*` environment
variables.

Given a Tracer, you will typically do one of two things:
 - report a transaction, or
 - report a span.

A transaction represents a top-level operation in your application, such as an
HTTP or RPC request. A span represents an operation within a transaction, such
as a database query, or a request to another service.

#### Transactions

To start a new transaction, you call `Tracer.StartTransaction` with the
transaction name and type. e.g.

```go
tx := elasticapm.DefaultTracer.StartTransaction("GET /api/v1", "request")
```

When the transaction has finished, you call `Transaction.Done` with the
duration, or supplying a negative value to have Done compute the duration
as `time.Now().Since(start)`. e.g.

```go
defer tx.Done(-1)
```

Once you have started a transaction, you can include it in a `context` object
for propagating throughout the application. e.g.

```go
ctx := elasticapm.ContextWithTransaction(ctx, tx)
```

The transaction can then be retrieved with `elasticapm.TransactionFromContext`:

```go
tx := elasticapm.TransactionFromContext(ctx)
```

#### Spans

To trace the execution of an operation within your transaction, you start
a span on that transaction using `Transaction.StartSpan`. Alternatively, if
you have a transaction within a `context` object, you can use `elasticapm.StartSpan`
instead. e.g.

```go
span, ctx := elasticapm.StartSpan(ctx, "span_name", "span_type")
if span != nil {
	defer span.Done(-1)
}
```

Note that both `Transaction.StartSpan` and `elasticapm.StartSpan` will return
a nil `Span` if the transaction is not being sampled. By default, all
transactions are sampled; that is, all transactions are sent with complete
detail to the Elastic APM server. If you configure the agent to sample
transactions at less than 100%, then spans and context will be dropped, and
in this case, StartSpan will sometimes return nil. Since sampling on the
DefaultTracer can be configured via an environment variable (`ELASTIC_APM_TRANSACTION_SAMPLE_RATE`),
it is a good idea to always allow for the result to be nil.

If a span is created using `elasticapm.StartSpan`, it will be included
in the resulting `context`. If you start a span using `Transaction.StartSpan`,
then you can add it to a `context` object using `elasticapm.ContextWithSpan`:

```go
ctx := elasticapm.ContextWithSpan(ctx, span)
```

Spans can be extracted from `context` using `elasticapm.SpanFromContext`:

```go
span := elasticapm.SpanFromContext(ctx)
```


#### Panic recovery and errors

If you want to recover panics, and report them along with your transaction,
you can use the `Tracer.Recover` or `Tracer.Recovered` methods. The former
should be used as a deferred call, while the latter can be used if you have
your own recovery logic. e.g.

```go
tx := elasticapm.DefaultTracer.StartTransaction(...)
defer tx.Done(-1)
defer elasticapm.DefaultTracer.Recover(tx)
```

(or)

```go
tx := elasticapm.DefaultTracer.StartTransaction(...)
defer tx.Done(-1)
defer func() {
	if v := recover(); v != nil {
		e := elasticapm.DefaultTracer.Recovered(v, tx)
		e.Send()
	}
	...
}()
```

If you use `Tracer.Recover`, the stack trace for the panic will be included
in the error data sent to Elastic APM. If the panic value is an error
created with [github.com/pkg/errors][], then its stack trace will be used;
otherwise the agent code will take a stack trace from the point of recovery.
If you use `Tracer.Recovered`, the stack trace is not set at all, and you
can set it using `Error.SetExceptionStacktrace` as necessary.

As well asfrom recovering from panics, you can also report errors using
`Tracer.NewError`. Given the resulting elasticapm.Error object, you can then
either set an "exception", or a log message. e.g.

```go
...
if err != nil {
	e := elasticapm.DefaultTracer.NewError()
	e.SetException(err)
	e.Exception.Handled = true // error was handled by the application
	e.Transaction = tx // optional; errors need not correspond to transactions
	e.Send()
}
```

If you are capturing errors in the context of a transaction which has been
added to a `context` object, then you can use the `elasticapm.CaptureError`
function:

```go
if err != nil {
	e := elasticapm.CaptureError(ctx, err)
	e.Send()
}
```

[Elastic APM]: https://www.elastic.co/solutions/apm
[github.com/pkg/errors]: https://github.com/pkg/errors
