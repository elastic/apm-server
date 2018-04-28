package apmlambda

import (
	"log"
	"net"
	"net/rpc"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"

	"github.com/aws/aws-lambda-go/lambda/messages"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

const (
	// TODO(axw) make this configurable via environment
	payloadLimit = 1024
)

var (
	// nonBlocking is passed to Tracer.Flush so it does not block functions.
	nonBlocking = make(chan struct{})

	// Globals below used during tracing, to avoid reallocating for each
	// invocation. Only one invocation will happen at a time.
	lambdaContext struct {
		RequestID       string `json:"request_id,omitempty"`
		Region          string `json:"region,omitempty"`
		XAmznTraceID    string `json:"x_amzn_trace_id,omitempty"`
		FunctionVersion string `json:"function_version,omitempty"`
		MemoryLimit     int    `json:"memory_limit,omitempty"`
		Request         string `json:"request,omitempty"`
		Response        string `json:"response,omitempty"`
	}
)

func init() {
	close(nonBlocking)
	lambdaContext.FunctionVersion = lambdacontext.FunctionVersion
	lambdaContext.MemoryLimit = lambdacontext.MemoryLimitInMB
	lambdaContext.Region = os.Getenv("AWS_REGION")

	if elasticapm.DefaultTracer.Service.Framework == nil {
		executionEnv := os.Getenv("AWS_EXECUTION_ENV")
		version := strings.TrimPrefix(executionEnv, "AWS_Lambda_")
		elasticapm.DefaultTracer.Service.Framework = &model.Framework{
			Name:    "AWS Lambda",
			Version: version,
		}
	}
}

// Function is type exposed via net/rpc, to match the signature implemented
// by the aws-lambda-go package.
type Function struct {
	client *rpc.Client
	tracer *elasticapm.Tracer
}

// Ping pings the function implementation.
func (f *Function) Ping(req *messages.PingRequest, response *messages.PingResponse) error {
	return f.client.Call("Function.Ping", req, response)
}

// Invoke invokes the Lambda function. This is our main trace point.
func (f *Function) Invoke(req *messages.InvokeRequest, response *messages.InvokeResponse) error {
	tx := f.tracer.StartTransaction(lambdacontext.FunctionName, "function")
	defer f.tracer.Flush(nonBlocking)
	defer tx.Done(-1)
	defer f.tracer.Recover(tx)
	if tx.Sampled() {
		tx.Context.SetCustom("lambda", &lambdaContext)
	}

	lambdaContext.RequestID = req.RequestId
	lambdaContext.XAmznTraceID = req.XAmznTraceId
	lambdaContext.Request = formatPayload(req.Payload)
	lambdaContext.Response = ""

	err := f.client.Call("Function.Invoke", req, response)
	if err != nil {
		e := f.tracer.NewError(err)
		e.Context.SetCustom("lambda", &lambdaContext)
		e.Transaction = tx
		e.Send()
		return err
	}

	if response.Payload != nil {
		lambdaContext.Response = formatPayload(response.Payload)
	}
	if response.Error != nil {
		e := f.tracer.NewError(invokeResponseError{response.Error})
		e.Context.SetCustom("lambda", &lambdaContext)
		e.Transaction = tx
		e.Send()
	}
	return nil
}

type invokeResponseError struct {
	err *messages.InvokeResponse_Error
}

func (e invokeResponseError) Error() string {
	return e.err.Message
}

func (e invokeResponseError) Type() string {
	return e.err.Type
}

func (e invokeResponseError) StackTrace() []stacktrace.Frame {
	frames := make([]stacktrace.Frame, len(e.err.StackTrace))
	for i, f := range e.err.StackTrace {
		frames[i] = stacktrace.Frame{
			Function: f.Label,
			File:     f.Path,
			Line:     int(f.Line),
		}
	}
	return frames
}

func formatPayload(payload []byte) string {
	if len(payload) > payloadLimit {
		payload = payload[:payloadLimit]
	}
	if !utf8.Valid(payload) {
		return ""
	}
	return string(payload)
}

func init() {
	pipeClient, pipeServer := net.Pipe()
	rpcClient := rpc.NewClient(pipeClient)
	go rpc.DefaultServer.ServeConn(pipeServer)

	origPort := os.Getenv("_LAMBDA_SERVER_PORT")
	lis, err := net.Listen("tcp", "localhost:"+origPort)
	if err != nil {
		log.Fatal(err)
	}
	srv := rpc.NewServer()
	srv.Register(&Function{
		client: rpcClient,
		tracer: elasticapm.DefaultTracer,
	})
	go srv.Accept(lis)

	// Setting _LAMBDA_SERVER_PORT causes lambda.Start
	// to listen on any free port. We don't care which;
	// we don't use it.
	os.Setenv("_LAMBDA_SERVER_PORT", "0")
}

// TODO(axw) Start() function, which wraps a given function
// such that its context is updated with the transaction.
