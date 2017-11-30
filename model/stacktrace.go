package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []StacktraceFrame

func (st *Stacktrace) Transform(service Service, smapAccessor utility.SmapAccessor) []common.MapStr {
	var frames []common.MapStr

	for _, fr := range *st {
		frame := fr.Transform(service, smapAccessor)
		frames = append(frames, frame)
	}
	return frames
}
