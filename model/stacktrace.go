package model

import (
	"github.com/elastic/beats/libbeat/common"
)

func TransformStacktrace(frames []StacktraceFrame, app App) []common.MapStr {
	var stacktrace []common.MapStr

	for _, fr := range frames {
		frame := TransformFrame(&fr, app)
		stacktrace = append(stacktrace, frame)
	}
	return stacktrace
}
