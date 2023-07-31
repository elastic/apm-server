package r8

import (
	"compress/zlib"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const mapIndex = ".apm-android-map"

var errFetcherUnavailable = errors.New("fetcher unavailable")

type MapFetcher struct {
	client *elasticsearch.Client
	logger *logp.Logger
}

// NewMapFetcher returns a MapFetcher.
func NewMapFetcher(c *elasticsearch.Client) *MapFetcher {
	logger := logp.NewLogger(logs.R8)
	return &MapFetcher{c, logger}
}

// Fetch fetches an R8 map from Elasticsearch.
func (p *MapFetcher) Fetch(ctx context.Context, name, version string) ([]byte, error) {
	resp, err := p.runSearchQuery(ctx, name, version)
	if err != nil {
		var networkErr net.Error
		if errors.As(err, &networkErr) {
			return nil, fmt.Errorf("failed to reach elasticsearch: %w: %v ", errFetcherUnavailable, err)
		}
		return nil, fmt.Errorf("failure querying ES: %w", err)
	}
	defer resp.Body.Close()

	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read ES response body: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			// http.StatusNotFound -> the index is missing
			// http.StatusForbidden -> we don't have permission to read from the index
			// In both cases we consider the fetcher unavailable so that APM Server can
			// fallback to other fetchers
			return nil, fmt.Errorf("%w: %s: %s", errFetcherUnavailable, resp.Status(), string(b))
		}
		return nil, fmt.Errorf("ES returned unknown status code: %s", resp.Status())
	}

	r, err := zlib.NewReader(base64.NewDecoder(base64.StdEncoding, resp.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer r.Close()

	uncompressedBody, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read sourcemap content: %w", err)
	}

	if uncompressedBody != nil && len(uncompressedBody) < 1 {
		return nil, nil
	}

	return uncompressedBody, nil
}

func (p *MapFetcher) runSearchQuery(ctx context.Context, name, version string) (*esapi.Response, error) {
	id := name + "-" + version + "-android"
	req := esapi.GetRequest{
		Index:      mapIndex,
		DocumentID: url.PathEscape(id),
	}
	return req.Do(ctx, p.client)
}
