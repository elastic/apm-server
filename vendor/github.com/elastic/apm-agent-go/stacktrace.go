package elasticapm

import (
	"path/filepath"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

func appendModelStacktraceFrames(out []model.StacktraceFrame, in []stacktrace.Frame) []model.StacktraceFrame {
	for _, f := range in {
		out = append(out, modelStacktraceFrame(f))
	}
	return out
}

func modelStacktraceFrame(in stacktrace.Frame) model.StacktraceFrame {
	var abspath string
	file := in.File
	if file != "" {
		if filepath.IsAbs(file) {
			abspath = file
		}
		file = filepath.Base(file)
	}
	packagePath, function := stacktrace.SplitFunctionName(in.Function)
	return model.StacktraceFrame{
		AbsolutePath: abspath,
		File:         file,
		Line:         in.Line,
		Function:     function,
		Module:       packagePath,
		LibraryFrame: stacktrace.IsLibraryPackage(packagePath),
	}
}
