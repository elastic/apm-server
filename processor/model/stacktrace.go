package model

import (
	"github.com/elastic/beats/libbeat/common"
)

type StacktraceFrames []StacktraceFrame
type Stacktrace struct {
	Frames         StacktraceFrames
	TransformFrame TransformStacktraceFrame
}

type TransformStacktrace func(s *Stacktrace) []common.MapStr

func (s *Stacktrace) Transform() []common.MapStr {
	var stacktrace []common.MapStr

	for _, fr := range s.Frames {
		frame := s.transformFrame(&fr)
		stacktrace = append(stacktrace, frame)
	}
	return stacktrace
}

func (s *Stacktrace) transformFrame(fr *StacktraceFrame) common.MapStr {
	if s.TransformFrame == nil {
		s.TransformFrame = (*StacktraceFrame).Transform
	}
	return s.TransformFrame(fr)
}
