package sourcemap

import (
	"fmt"
	"time"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/beats/libbeat/logp"
)

type Accessor interface {
	Fetch(id Id) (*sourcemap.Consumer, error)
	Remove(id Id)
}

type SmapAccessor struct {
	es    elasticsearch
	cache *cache
}

func NewSmapAccessor(config Config) (*SmapAccessor, error) {
	logp.NewLogger("sourcemap").Debugf("NewSmapAccessor created at Time.now %v for index", time.Now().Unix(), config.Index)

	es, err := NewElasticsearch(config.ElasticsearchConfig, config.Index)
	if err != nil {
		return nil, err
	}
	cache, err := newCache(config.CacheExpiration)
	if err != nil {
		return nil, err
	}
	return &SmapAccessor{
		es:    es,
		cache: cache,
	}, nil
}

func (s *SmapAccessor) Fetch(id Id) (*sourcemap.Consumer, error) {
	if !id.Valid() {
		return nil, Error{
			Msg:  fmt.Sprintf("Sourcemap Key Error for %v", id.String()),
			Kind: KeyError,
		}
	}
	consumer, found := s.cache.fetch(id)
	if consumer != nil {
		// avoid fetching ES again when key was already queried
		// but no Sourcemap was found for it.
		return consumer, nil
	}
	if found {
		return nil, errSmapNotAvailable(id)
	}
	consumer, err := s.es.fetch(id)
	if err != nil {
		return nil, err
	}
	s.cache.add(id, consumer)
	if consumer == nil {
		return nil, errSmapNotAvailable(id)
	}
	return consumer, nil
}

func (s *SmapAccessor) Remove(id Id) {
	s.cache.remove(id)
}

func errSmapNotAvailable(id Id) Error {
	return Error{
		Msg:  fmt.Sprintf("No Sourcemap available for %v.", id.String()),
		Kind: MapError,
	}
}
