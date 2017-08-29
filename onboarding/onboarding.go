package onboarding

import (
	"sync"
	"time"

	"github.com/elastic/apm-server/processor"
	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

var once sync.Once

const (
	processorName = "onboarding"
)

func NotifyUp(isServerUp func() bool, publish func(events []beat.Event)) {
	once.Do(func() {
		if isServerUp() {
			logp.Info("Publishing onboarding document")
			docMappings := []m.DocMapping{
				{Key: "processor", Apply: func() common.MapStr {
					return common.MapStr{"name": processorName, "event": processorName}
				}},
			}
			events := []beat.Event{processor.CreateDoc(time.Now(), docMappings)}
			publish(events)
		}
	})
}
