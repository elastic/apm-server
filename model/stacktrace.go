package model

import (
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []StacktraceFrame

func (st *Stacktrace) Transform() []common.MapStr {
	var frames []common.MapStr

	for _, fr := range *st {
		frame := fr.Transform()
		frames = append(frames, frame)
	}
	return frames
}
