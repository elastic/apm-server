package beatertest

import (
	"io"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
)

type MockESTransport struct {
	RoundTripFn func(req *http.Request) (*http.Response, error)
	Executed    int
}

func (t *MockESTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.Executed++
	return t.RoundTripFn(req)
}

func MockESDefaultClient(statusCode int, body io.ReadCloser) (*elasticsearch.Client, *MockESTransport, error) {
	transport := &MockESTransport{
		RoundTripFn: func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: statusCode, Body: body}, nil
		},
	}
	client, err := MockESClient(transport)
	return client, transport, err
}

func MockESClient(transport *MockESTransport) (*elasticsearch.Client, error) {
	return elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{}, Transport: transport})
}
