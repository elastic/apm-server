package model

import (
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace struct {
	Frames []StacktraceFrame
}

func (st *Stacktrace) Transform() common.MapStr {
	if st == nil || len((*st).Frames) == 0 {
		return nil
	}

	var frames []common.MapStr
	onlyLib := true

	for _, fr := range (*st).Frames {
		frame := fr.Transform()
		frames = append(frames, frame)
		onlyLib = onlyLib && fr.Library()
	}

	return common.MapStr{
		"frames":              frames,
		"only_library_frames": onlyLib,
	}
}
