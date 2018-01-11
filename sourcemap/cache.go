package sourcemap

import (
	"time"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"

	"github.com/elastic/beats/libbeat/logp"
)

type cache struct {
	goca *gocache.Cache
}

func newCache(expiration, cleanupInterval time.Duration) (*cache, error) {
	if expiration < 0 || cleanupInterval < 0 {
		return nil, Error{
			Msg:  "Cache cannot be initialized. Expiration and CleanupInterval need to be > 0",
			Kind: InitError,
		}
	}
	return &cache{goca: gocache.New(expiration, cleanupInterval)}, nil
}

func (c *cache) add(id Id, consumer *sourcemap.Consumer) {
	c.goca.Set(id.Key(), consumer, gocache.DefaultExpiration)
	logp.Debug("sourcemap", "Added id %v. Cache now has %v entries.", id.Key(), c.goca.ItemCount())
}

func (c *cache) remove(id Id) {
	c.goca.Delete(id.Key())
	logp.Debug("sourcemap", "Removed id %v. Cache now has %v entries.", id.Key(), c.goca.ItemCount())
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
