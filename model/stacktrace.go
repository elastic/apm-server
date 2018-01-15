package model

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []*StacktraceFrame

func (st Stacktrace) Transform(config *pr.Config, service Service) []common.MapStr {
	var fr *StacktraceFrame
	var frames []common.MapStr
	noFrames := len(st)
	frames = make([]common.MapStr, noFrames)

	fct := "<anonymous>"
	for idx := noFrames - 1; idx >= 0; idx-- {
		fr = st[idx]
		fr.SourcemapFunction = fct
		frames[idx] = fr.Transform(config, service)
		fct = fr.SourcemapFunction
	}
	return frames
}
