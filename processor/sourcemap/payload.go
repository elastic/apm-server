package sourcemap

import (
	"time"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var sourcemapCounter = monitoring.NewInt(sourcemapUploadMetrics, "counter")

type payload struct {
	App            m.AppCore `json:"app"`
	Sourcemap      sourcemap `json:"sourcemap"`
	BundleFilepath string    `json:"bundle_filepath"`
}

type sourcemap struct {
	Version        int      `json:"version"`
	Sources        []string `json:"sources"`
	Names          []string `json:"names"`
	Mappings       string   `json:"mappings"`
	File           *string  `json:"file"`
	SourcesContent []string `json:"sourcesContent"`
	SourceRoot     *string  `json:"sourceRoot"`
}

func (pa *payload) transform() []beat.Event {
	var events []beat.Event = []beat.Event{pr.CreateDoc(mappings(pa))}
	sourcemapCounter.Add(1)
	return events
}

func mappings(pa *payload) (time.Time, []m.DocMapping) {
	return time.Now(),
		[]m.DocMapping{
			{Key: "processor", Apply: func() common.MapStr {
				return common.MapStr{"name": processorName, "event": processorName}
			}},
			{Key: processorName, Apply: func() common.MapStr {
				return common.MapStr{
					"bundle_filepath": pa.BundleFilepath,
					"app":             pa.App.Transform(),
					"sourcemap":       pa.Sourcemap.transform(),
				}
			}},
		}
}

func (sourcemap *sourcemap) transform() common.MapStr {
	enh := utility.NewMapStrEnhancer()
	m := common.MapStr{}
	enh.Add(m, "version", sourcemap.Version)
	enh.Add(m, "file", sourcemap.File)
	enh.Add(m, "mappings", sourcemap.Mappings)
	enh.Add(m, "names", sourcemap.Names)
	enh.Add(m, "sources", sourcemap.Sources)
	enh.Add(m, "sourceRoot", sourcemap.SourceRoot)
	enh.Add(m, "sourcesContent", sourcemap.SourcesContent)
	return m
}
