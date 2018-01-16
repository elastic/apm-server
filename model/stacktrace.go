package model

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []*StacktraceFrame

func (st Stacktrace) Transform(config *pr.Config, service Service) []common.MapStr {
	if config == nil || config.SmapMapper == nil {
		return st.transform(config)
	}
	return st.sourcemapAndTransform(config, service)
}

func (st Stacktrace) transform(config *pr.Config) []common.MapStr {
	var frames []common.MapStr
	for _, fr := range st {
		frame := fr.Transform(config)
		frames = append(frames, frame)
	}
	return frames
}

// source map algorithm:
// apply source mapping frame by frame
// if no source map could be found, set updated to false and set sourcemap error
// otherwise use source map library for mapping and update
// - filename: only if it was found
// - function:
//   * should be moved down one stack trace frame,
//   * the function name of the first frame is set to <anonymous>
//   * if one frame is not found in the source map, this frame is left out and
//   the function name from the previous frame is set for the one but next frame
// - colno
// - lineno
// - abs_path is set to the cleaned abs_path
// - sourcmeap.updated is set to true
func (st Stacktrace) sourcemapAndTransform(config *pr.Config, service Service) []common.MapStr {
	noFrames := len(st)
	if noFrames == 0 {
		return nil
	}
	var fr *StacktraceFrame
	var frames []common.MapStr
	frames = make([]common.MapStr, noFrames)

	fct := "<anonymous>"
	for idx := noFrames - 1; idx >= 0; idx-- {
		fr = st[idx]
		fct = fr.applySourcemap(config.SmapMapper, service, fct)
		frames[idx] = fr.Transform(config)
	}
	return frames
}
