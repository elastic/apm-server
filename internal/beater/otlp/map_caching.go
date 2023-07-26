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

// NewMapCachingFetcher returns a MapCachingFetcher that wraps backend, caching results for the configured cacheExpiration.
func NewMapCachingFetcher(
	backend MapFetcher,
	cacheSize int,
	invalidationChan <-chan []sourcemap.Identifier,
) (*MapCachingFetcher, error) {
	logger := logp.NewLogger(logs.Sourcemap)

	lruCache, err := lru.NewWithEvict(cacheSize, func(key, value interface{}) {
		logger.Debugf("Removed id %v", key)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create lru cache for map caching fetcher: %w", err)
	}

	go func() {
		logger.Debug("listening for invalidation...")

		for arr := range invalidationChan {
			for _, id := range arr {
				logger.Debugf("Invalidating id %v", id)
				lruCache.Remove(id)
			}
		}
	}()

	return &MapCachingFetcher{
		cache:   lruCache,
		backend: backend,
		logger:  logger,
	}, nil
}

// Fetch fetches an android source map from the cache or wrapped backend.
func (f MapCachingFetcher) Fetch(ctx context.Context, name, version string) ([]byte, error) {
	key := sourcemap.Identifier{
		Name:    name,
		Version: version,
		Path:    "android",
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

func (f *MapCachingFetcher) add(key sourcemap.Identifier, data []byte) {
	f.cache.Add(key, data)
	f.logger.Debugf("Added id %v. Cache now has %v entries.", key, f.cache.Len())
}
