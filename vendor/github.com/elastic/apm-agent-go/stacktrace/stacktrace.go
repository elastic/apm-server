package stacktrace

import (
	"runtime"
	"strings"
)

//go:generate /bin/bash generate_library.bash std ..

// TODO(axw) add a function for marking frames as library
// frames, based on configuration. Possibly use
// in-the-same-repo as a heuristic?

// AppendStacktrace appends at most n entries to frames,
// skipping skip frames starting with AppendStacktrace,
// and returns the extended slice. If n is negative, then
// all stack frames will be appended.
//
// See RuntimeFrame for information on what details are included.
func AppendStacktrace(frames []Frame, skip, n int) []Frame {
	if n == 0 {
		return frames
	}
	var pc []uintptr
	if n > 0 {
		pc = make([]uintptr, n)
		pc = pc[:runtime.Callers(skip+1, pc)]
	} else {
		// n is negative, get all frames.
		n = 0
		pc = make([]uintptr, 10)
		for {
			n += runtime.Callers(skip+n+1, pc[n:])
			if n < len(pc) {
				pc = pc[:n]
				break
			}
			pc = append(pc, 0)
		}
	}
	return AppendCallerFrames(frames, pc)
}

// AppendCallerFrames appends to frames for the PCs in callers,
// and returns the extended slice.
//
// See RuntimeFrame for information on what details are included.
func AppendCallerFrames(frames []Frame, callers []uintptr) []Frame {
	if len(callers) == 0 {
		return frames
	}
	runtimeFrames := runtime.CallersFrames(callers)
	for {
		runtimeFrame, more := runtimeFrames.Next()
		frames = append(frames, RuntimeFrame(runtimeFrame))
		if !more {
			break
		}
	}
	return frames
}

// RuntimeFrame returns a Frame based on the given runtime.Frame.
//
// The resulting Frame will have the file path, package-qualified
// function name, and line number set. The function name can be
// split using SplitFunctionName, and the absolute path of the
// file and its base name can be determined using standard filepath
// functions.
func RuntimeFrame(in runtime.Frame) Frame {
	return Frame{
		File:     in.File,
		Function: in.Function,
		Line:     in.Line,
	}
}

// SplitFunctionName splits the function name as formatted in
// runtime.Frame.Function, and returns the package path and
// function name components.
func SplitFunctionName(in string) (packagePath, function string) {
	function = in
	if function == "" {
		return "", ""
	}
	// The last part of a package path will always have "."
	// encoded as "%2e", so we can pick off the package path
	// by finding the last part of the package path, and then
	// the proceeding ".".
	//
	// Unexported method names may contain the package path.
	// In these cases, the method receiver will be enclosed
	// in parentheses, so we can treat that as the start of
	// the function name.
	sep := strings.Index(function, ".(")
	if sep >= 0 {
		packagePath = unescape(function[:sep])
		function = function[sep+1:]
	} else {
		offset := 0
		if sep := strings.LastIndex(function, "/"); sep >= 0 {
			offset = sep
		}
		if sep := strings.IndexRune(function[offset+1:], '.'); sep >= 0 {
			packagePath = unescape(function[:offset+1+sep])
			function = function[offset+1+sep+1:]
		}
	}
	return packagePath, function
}

func unescape(s string) string {
	var n int
	for i := 0; i < len(s); i++ {
		if s[i] == '%' {
			n++
		}
	}
	if n == 0 {
		return s
	}
	bytes := make([]byte, 0, len(s)-2*n)
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '%' && i+2 < len(s) {
			b = fromhex(s[i+1])<<4 | fromhex(s[i+2])
			i += 2
		}
		bytes = append(bytes, b)
	}
	return string(bytes)
}

func fromhex(b byte) byte {
	if b >= 'a' {
		return 10 + b - 'a'
	}
	return b - '0'
}
