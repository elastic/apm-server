package otlp

import (
	"context"
	"errors"
	"fmt"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/sourcemap"
	"github.com/elastic/elastic-agent-libs/logp"
	lru "github.com/hashicorp/golang-lru"
)

type MapCachingFetcher struct {
	cache   *lru.Cache
	backend MapFetcher
	logger  *logp.Logger
}

type identifier struct {
	Name    string
	Version string
}

// NewMapCachingFetcher returns a MapCachingFetcher that wraps backend, caching results for the configured cacheExpiration.
func NewMapCachingFetcher(
	backend MapFetcher,
	cacheSize int,
) (*MapCachingFetcher, error) {
	logger := logp.NewLogger(logs.Sourcemap)

	lruCache, err := lru.NewWithEvict(cacheSize, func(key, value interface{}) {
		logger.Debugf("Removed id %v", key)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create lru cache for map caching fetcher: %w", err)
	}

	return &MapCachingFetcher{
		cache:   lruCache,
		backend: backend,
		logger:  logger,
	}, nil
}

// Fetch fetches an android source map from the cache or wrapped backend.
func (f *MapCachingFetcher) Fetch(ctx context.Context, name, version string) ([]byte, error) {
	key := identifier{
		Name:    name,
		Version: version,
	}

	// fetch from cache
	if val, found := f.cache.Get(key); found {
		consumer, _ := val.([]byte)
		return consumer, nil
	}

	// fetch from the store and ensure caching for all non-temporary results
	data, err := f.backend.Fetch(ctx, name, version)
	if err != nil {
		if errors.Is(err, sourcemap.ErrMalformedSourcemap) {
			f.add(key, nil)
		}
		return nil, err
	}
	f.add(key, data)
	return data, nil
}

func (f *MapCachingFetcher) add(key identifier, data []byte) {
	f.cache.Add(key, data)
	f.logger.Debugf("Added id %v. Cache now has %v entries.", key, f.cache.Len())
}
