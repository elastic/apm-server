package beater

import (
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

func notifyListening(config *Config, pubFct func(beat.Event)) {

	var isServerUp = func() bool {
		secure := config.SSL.isEnabled()
		return isServerUp(secure, config.Host, 10, time.Second)
	}

	if isServerUp() {
		logp.NewLogger("onboarding").Info("Publishing onboarding document")

		event := beat.Event{
			Timestamp: time.Now(),
			Fields:    common.MapStr{"listening": config.Host},
		}
		pubFct(event)
	}
}
