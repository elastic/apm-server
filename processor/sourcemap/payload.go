package sourcemap

import (
	"time"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var sourcemapCounter = monitoring.NewInt(sourcemapUploadMetrics, "counter")

type payload struct {
	ServiceName    string        `json:"service_name"`
	ServiceVersion string        `json:"service_version"`
	Sourcemap      common.MapStr `json:"sourcemap"`
	BundleFilepath string        `json:"bundle_filepath"`
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
					"service":         common.MapStr{"name": pa.ServiceName, "version": pa.ServiceVersion},
					"sourcemap":       pa.Sourcemap,
				}
			}},
		}
}
