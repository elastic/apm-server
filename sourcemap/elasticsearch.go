package sourcemap

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	es "github.com/elastic/beats/libbeat/outputs/elasticsearch"
)

type elasticsearch interface {
	fetch(id Id) (*sourcemap.Consumer, error)
}

type smapElasticsearch struct {
	mu      sync.Mutex // guards clients
	clients []es.Client

	index string
}

func NewElasticsearch(config *common.Config, index string) (*smapElasticsearch, error) {
	esClients, err := es.NewElasticsearchClients(config)
	if err != nil || esClients == nil || len(esClients) == 0 {
		return nil, Error{Msg: fmt.Sprintf("Sourcemap ES Client cannot be initialized. %v", err.Error()), Kind: InitError}
	}
	if index == "" {
		index = "*"
	}
	return &smapElasticsearch{clients: esClients, index: index}, nil
}

func (e *smapElasticsearch) fetch(id Id) (*sourcemap.Consumer, error) {
	result, err := e.runESQuery(query(id))
	if err != nil {
		return nil, err
	}
	return parseResult(result, id)
}

func (e *smapElasticsearch) runESQuery(body map[string]interface{}) (*es.SearchResults, error) {
	var err error
	var result *es.SearchResults
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, client := range e.clients {
		_, result, err = client.Connection.SearchURIWithBody(e.index, "", nil, body)
		if err == nil {
			return result, nil
		}
	}
	if err != nil {
		return nil, Error{Msg: err.Error(), Kind: AccessError}
	}
	return result, nil
}

func parseResult(result *es.SearchResults, id Id) (*sourcemap.Consumer, error) {
	if result.Hits.Total == 0 {
		return nil, nil
	}
	if result.Hits.Total > 1 {
		logp.NewLogger("sourcemap").Warnf("Multiple sourcemaps found for service %s version %s and file %s , fetching the last uploaded one",
			id.ServiceName, id.ServiceVersion, id.Path)
	}
	smap, err := parseSmap(result.Hits.Hits[0])
	if err != nil {
		return nil, err
	}
	cons, err := sourcemap.Parse("", []byte(smap))
	if err != nil {
		return nil, Error{
			Msg:  fmt.Sprintf("Could not parse Sourcemap. %v", err.Error()),
			Kind: ParseError,
		}
	}
	return cons, nil
}

func query(id Id) map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"processor.name": "sourcemap"}},
					{"term": map[string]interface{}{"sourcemap.bundle_filepath": id.Path}},
					{"term": map[string]interface{}{"sourcemap.service.name": id.ServiceName}},
					{"term": map[string]interface{}{"sourcemap.service.version": id.ServiceVersion}},
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
		return "", Error{Msg: err.Error(), Kind: ParseError}
	}
	// until https://github.com/golang/go/issues/19858 is resolved
	if smap.Source.Sourcemap.Sourcemap == "" {
		return "", Error{Msg: "Sourcemapping ES Result not in expected format", Kind: ParseError}
	}
	return smap.Source.Sourcemap.Sourcemap, nil
}
