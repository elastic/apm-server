package model

import (
	"github.com/elastic/apm-server/config"
	"github.com/elastic/beats/libbeat/beat"
)

type Transformable interface {
	Transform(config.TransformConfig, *TransformContext) beat.Event
}

type TransformableBatch []Transformable

func (tb *TransformableBatch) Transform(c config.TransformConfig, tctx *TransformContext) []beat.Event {
	l := len(*tb)
	events := make([]beat.Event, l)
	for idx, t := range *tb {
		events[idx] = t.Transform(c, tctx)
	}
	return events
}
