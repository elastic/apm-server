package model

import (
	"errors"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/beats/libbeat/common"
)

type Stacktrace []*StacktraceFrame

func DecodeStacktrace(input interface{}, err error) (*Stacktrace, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.([]interface{})
	if !ok {
		return nil, errors.New("Invalid type for stacktrace")
	}
	st := make(Stacktrace, len(raw))
	for idx, fr := range raw {
		st[idx], err = DecodeStacktraceFrame(fr, err)
	}
	return &st, err
}

func (st *Stacktrace) Transform(config config.Config, service Service) []common.MapStr {
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
	frameCount := len(*st)
	if frameCount == 0 {
		return nil
	}
	var fr *StacktraceFrame
	var frames []common.MapStr
	frames = make([]common.MapStr, frameCount)

	fct := "<anonymous>"
	for idx := frameCount - 1; idx >= 0; idx-- {
		fr = (*st)[idx]
		if config.SmapMapper != nil {
			fct = fr.applySourcemap(config.SmapMapper, service, fct)
		}
		frames[idx] = fr.Transform(config)
	}
	return frames
}
