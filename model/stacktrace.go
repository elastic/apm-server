package model

import (
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []StacktraceFrame

func (st *Stacktrace) Transform() ([]common.MapStr, bool) {
	var frames []common.MapStr
	onlyLib := true

	for _, fr := range *st {
		frame := fr.Transform()
		frames = append(frames, frame)
		onlyLib = onlyLib && fr.Library()
	}
	return frames, onlyLib
}
