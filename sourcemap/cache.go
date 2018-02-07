package sourcemap

import (
	"math"
	"time"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"

	"github.com/elastic/beats/libbeat/logp"
)

const (
	MinCleanupIntervalSeconds float64 = 60
	LoggerSelector                    = "sourcemap"
)

type cache struct {
	goca *gocache.Cache
}

func newCache(expiration time.Duration) (*cache, error) {
	if expiration < 0 {
		return nil, Error{
			Msg:  "Cache cannot be initialized. Expiration and CleanupInterval need to be >= 0",
			Kind: InitError,
		}
	}
	return &cache{goca: gocache.New(expiration, cleanupInterval(expiration))}, nil
}

func (c *cache) add(id Id, consumer *sourcemap.Consumer) {
	c.goca.Set(id.Key(), consumer, gocache.DefaultExpiration)
	logp.NewLogger(LoggerSelector).Debugf("Added id %v. Cache now has %v entries.", id.Key(), c.goca.ItemCount())
}

func (c *cache) remove(id Id) {
	c.goca.Delete(id.Key())
	logp.NewLogger(LoggerSelector).Debugf("Removed id %v. Cache now has %v entries.", id.Key(), c.goca.ItemCount())
}

func (c *cache) fetch(id Id) (*sourcemap.Consumer, bool) {
	if cached, found := c.goca.Get(id.Key()); found {
		if cached == nil {
			// in case empty value was cached
			// return found=true
			return nil, true
		}
		return cached.(*sourcemap.Consumer), true
	}
	return nil, false
}

func cleanupInterval(ttl time.Duration) time.Duration {
	return time.Duration(math.Max(ttl.Seconds(), MinCleanupIntervalSeconds)) * time.Second
}
