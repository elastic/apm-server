package r8

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const mapIndex = ".apm-android-map"

var errFetcherUnvailable = errors.New("fetcher unavailable")

type MapFetcher struct {
	client *elasticsearch.Client
}

// NewMapFetcher returns a MapFetcher.
func NewMapFetcher(c *elasticsearch.Client) *MapFetcher {
	return &MapFetcher{c}
}

// Fetch fetches an R8 map from Elasticsearch.
func (p *MapFetcher) Fetch(ctx context.Context, name, version string) (io.ReadCloser, error) {
	resp, err := p.runSearchQuery(ctx, name, version)
	if err != nil {
		var networkErr net.Error
		if errors.As(err, &networkErr) {
			return nil, fmt.Errorf("failed to reach elasticsearch: %w: %v ", errFetcherUnvailable, err)
		}
		return nil, fmt.Errorf("failure querying ES: %w", err)
	}

	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read ES response body: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			// http.StatusNotFound -> the index is missing
			// http.StatusForbidden -> we don't have permission to read from the index
			// In both cases we consider the fetcher unavailable so that APM Server can
			// fallback to other fetchers
			return nil, fmt.Errorf("%w: %s: %s", errFetcherUnvailable, resp.Status(), string(b))
		}
		return nil, fmt.Errorf("ES returned unknown status code: %s", resp.Status())
	}

	return resp.Body, nil
}

func (p *MapFetcher) runSearchQuery(ctx context.Context, name, version string) (*esapi.Response, error) {
	id := name + "-" + version
	req := esapi.GetRequest{
		Index:      mapIndex,
		DocumentID: url.PathEscape(id),
	}
	return req.Do(ctx, p.client)
}
