package onboarding

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

func TestNotifyUpOnce(t *testing.T) {
	var serverUp = func() bool { return true }
	var saved []beat.Event
	var publish = func(events []beat.Event) { saved = append(saved, events...) }

	NotifyUp(serverUp, publish)
	NotifyUp(serverUp, publish)

	assert.Len(t, saved, 1)
	assert.WithinDuration(t, time.Now(), saved[0].Timestamp, time.Second)
	p := saved[0].Fields["processor"].(common.MapStr)
	assert.Equal(t, "onboarding", p["name"].(string))
	assert.Equal(t, "onboarding", p["event"].(string))
}

func TestNotifyUpServerDown(t *testing.T) {
	var serverUp = func() bool { return false }
	var saved []beat.Event
	var publish = func(events []beat.Event) { saved = append(saved, events...) }

	NotifyUp(serverUp, publish)

	assert.Nil(t, saved)
}
