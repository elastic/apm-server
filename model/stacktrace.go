package model

import (
	"errors"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace struct {
	Frames []*StacktraceFrame
}

func (st *Stacktrace) Decode(input interface{}) error {
	if input == nil || st == nil {
		return nil
	}
	raw, ok := input.([]interface{})
	if !ok {
		return errors.New("Invalid type for stacktrace")
	}

	st.Frames = make([]*StacktraceFrame, len(raw))
	for idx, fr := range raw {
		frame := StacktraceFrame{}
		err := frame.Decode(fr)
		if err != nil {
			return err
		}
		st.Frames[idx] = &frame
	}
	return nil
}

func (st *Stacktrace) Transform(config *pr.Config, service Service) []common.MapStr {
	if st == nil {
		return nil
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
	//   the function name from the previous frame is used
	//   * if a mapping could be applied but no function name is found, the
	//   function name for the next frame is set to <unknown>
	// - colno
	// - lineno
	// - abs_path is set to the cleaned abs_path
	// - sourcmeap.updated is set to true
	frameCount := len(st.Frames)
	if frameCount == 0 {
		return nil
	}
	var fr *StacktraceFrame
	var frames []common.MapStr
	frames = make([]common.MapStr, frameCount)

	fct := "<anonymous>"
	for idx := frameCount - 1; idx >= 0; idx-- {
		fr = st.Frames[idx]
		if config != nil && config.SmapMapper != nil {
			fct = fr.applySourcemap(config.SmapMapper, service, fct)
		}
		frames[idx] = fr.Transform(config)
	}
	return frames
}
