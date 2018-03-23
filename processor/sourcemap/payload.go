package sourcemap

import (
	"time"

	"github.com/elastic/apm-server/config"
	smap "github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	sourcemapCounter = monitoring.NewInt(sourcemapUploadMetrics, "counter")
	processorEntry   = common.MapStr{"name": processorName, "event": smapDocType}
)

type payload struct {
	ServiceName    string
	ServiceVersion string
	Sourcemap      string
	BundleFilepath string
}

func (pa *payload) transform(config config.Config) []beat.Event {
	sourcemapCounter.Add(1)
	if pa == nil {
		return nil
	}

	if config.SmapMapper == nil {
		logp.NewLogger("sourcemap").Error("Sourcemap Accessor is nil, cache cannot be invalidated.")
	} else {
		config.SmapMapper.NewSourcemapAdded(smap.Id{
			ServiceName:    pa.ServiceName,
			ServiceVersion: pa.ServiceVersion,
			Path:           pa.BundleFilepath,
		})
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor": processorEntry,
			smapDocType: common.MapStr{
				"bundle_filepath": pa.BundleFilepath,
				"service":         common.MapStr{"name": pa.ServiceName, "version": pa.ServiceVersion},
				"sourcemap":       pa.Sourcemap,
			},
		},
		Timestamp: time.Now(),
	}
	return []beat.Event{ev}
}
