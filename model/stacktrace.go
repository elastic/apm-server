package model

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []StacktraceFrame

func (st *Stacktrace) Transform(config *pr.Config, service Service) []common.MapStr {
	var frames []common.MapStr

	for _, fr := range *st {
		frame := fr.Transform(config, service)
		frames = append(frames, frame)
	}
	return frames
}
