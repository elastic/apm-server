package sourcemap

import (
	"time"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var sourcemapCounter = monitoring.NewInt(sourcemapUploadMetrics, "counter")

type payload struct {
	ServiceName    string `mapstructure:"service_name"`
	ServiceVersion string `mapstructure:"service_version"`
	Sourcemap      common.MapStr
	BundleFilepath string `mapstructure:"bundle_filepath"`
}

func (pa *payload) transform() []beat.Event {
	var events = []beat.Event{pr.CreateDoc(mappings(pa))}
	sourcemapCounter.Add(1)
	return events
}

func mappings(pa *payload) (time.Time, []utility.DocMapping) {
	return time.Now(),
		[]utility.DocMapping{
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
