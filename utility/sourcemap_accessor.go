package utility

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-sourcemap/sourcemap"
	cache "github.com/patrickmn/go-cache"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs/elasticsearch"
)

type SmapAccessor interface {
	Fetch(smapId SmapID) (*sourcemap.Consumer, error)
	RemoveFromCache(smapId SmapID)
}

type Enum string

const (
	InitError   Enum = "InitError"
	AccessError Enum = "AccessError"
	MapError    Enum = "MapError"
	ParseError  Enum = "ParseError"
)

type SmapError struct {
	Msg  string
	Kind Enum
}

func (e SmapError) Error() string {
	return fmt.Sprintf("%s", e.Msg)
}

type SmapID struct {
	ServiceName    string
	ServiceVersion string
	Path           string
}

type SmapConfig struct {
	CacheExpiration      time.Duration //seconds
	CacheCleanupInterval time.Duration //seconds
	ElasticsearchConfig  *common.Config
	Index                string
}

type SourcemapAccessor struct {
	esClients              []elasticsearch.Client
	smapCache              *cache.Cache
	cacheDefaultExpiration time.Duration
	index                  string
}

func NewSourcemapAccessor(config SmapConfig) (*SourcemapAccessor, error) {
	logp.Debug("sourcemap", "NewSourcemapAccessor created at Time.now %v for index", time.Now().Unix(), config.Index)
	esClients, err := elasticsearch.NewElasticsearchClients(config.ElasticsearchConfig)
	if err != nil || esClients == nil || len(esClients) == 0 {
		err := SmapError{Msg: "Sourcemap ESClient cannot be initialized.", Kind: InitError}
		logp.Err(err.Error())
		return nil, err
	}

	smapCache := cache.New(config.CacheExpiration, config.CacheCleanupInterval)
	return &SourcemapAccessor{
		esClients:              esClients,
		smapCache:              smapCache,
		cacheDefaultExpiration: config.CacheExpiration,
		index: fmt.Sprintf("%v*", config.Index),
	}, nil
}

func (s *SourcemapAccessor) Fetch(smapId SmapID) (*sourcemap.Consumer, error) {
	smapConsumer := s.fetchFromCache(smapId)
	if smapConsumer != nil {
		return smapConsumer, nil
	}
	smapConsumer, err := s.fetchFromES(smapId)
	if err != nil {
		return nil, err
	}
	s.AddToCache(smapId, smapConsumer)
	return smapConsumer, nil
}

func (s *SourcemapAccessor) AddToCache(smapId SmapID, smap *sourcemap.Consumer) {
	s.smapCache.Set(smapId.key(), smap, s.cacheDefaultExpiration)
	logp.Debug("sourcemap", "Added smapId %v. Cache now has %v entries.", smapId.key(), s.smapCache.ItemCount())
}

func (s *SourcemapAccessor) RemoveFromCache(smapId SmapID) {
	s.smapCache.Delete(smapId.key())
	logp.Debug("sourcemap", "Removed smapId %v. Cache now has %v entries.", smapId.key(), s.smapCache.ItemCount())
}

func (s *SourcemapAccessor) fetchFromCache(smapId SmapID) *sourcemap.Consumer {
	if cached, found := s.smapCache.Get(smapId.key()); found {
		return cached.(*sourcemap.Consumer)
	}
	return nil
}

func (s *SourcemapAccessor) fetchFromES(smapId SmapID) (*sourcemap.Consumer, error) {
	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"processor.name": "sourcemap"}},
					{"term": map[string]interface{}{"sourcemap.bundle_filepath": smapId.Path}},
					{"term": map[string]interface{}{"sourcemap.service.name": smapId.ServiceName}},
					{"term": map[string]interface{}{"sourcemap.service.version": smapId.ServiceVersion}},
				},
			},
		},
		"size": 1,
		"sort": []map[string]interface{}{
			{
				"@timestamp": map[string]interface{}{
					"order": "desc",
				},
			},
		},
		"_source": "sourcemap.sourcemap",
	}

	result, err := s.runESQuery(body)
	if err != nil {
		return nil, err
	}
	if result.Hits.Total == 0 {
		return nil, SmapError{Msg: fmt.Sprintf("No Sourcemap available for %v.", smapId.id()), Kind: MapError}
	}
	smap, err := parseSmap(result.Hits.Hits[0])
	if err != nil {
		return nil, err
	}
	cons, err := sourcemap.Parse("", []byte(smap))
	if err != nil {
		return nil, SmapError{Msg: err.Error(), Kind: MapError}
	}
	return cons, nil
}

func (s *SourcemapAccessor) runESQuery(body map[string]interface{}) (*elasticsearch.SearchResults, error) {
	var err error
	var result *elasticsearch.SearchResults
	for _, client := range s.esClients {
		_, result, err = client.Connection.SearchURIWithBody(s.index, "", nil, body)
		if err == nil {
			return result, nil
		}
	}
	if err != nil {
		return nil, SmapError{Msg: err.Error(), Kind: AccessError}
	}
	return result, nil
}

func parseSmap(result []byte) (string, error) {
	var smap struct {
		Source struct {
			Sourcemap struct {
				Sourcemap string
			}
		} `json:"_source"`
	}
	err := json.Unmarshal(result, &smap)
	if err != nil {
		return "", SmapError{Msg: err.Error(), Kind: ParseError}
	}
	// until https://github.com/golang/go/issues/19858 is resolved
	if smap.Source.Sourcemap.Sourcemap == "" {
		return "", SmapError{Msg: "Sourcemapping ES Result not in expected format", Kind: ParseError}
	}
	return smap.Source.Sourcemap.Sourcemap, nil
}

func (s *SmapID) key() string {
	return strings.Join([]string{s.ServiceName, s.ServiceVersion, s.Path}, "_")
}

func (s *SmapID) id() string {
	return fmt.Sprintf("Service Name: %s, Service Version: %s and Path: %s.",
		s.ServiceName,
		s.ServiceVersion,
		s.Path)
}
