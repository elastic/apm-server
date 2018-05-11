package stacktrace

// Frame describes a stack frame.
type Frame struct {
	// File is the filename of the location of the stack frame.
	// This may be either the absolute or base name of the file.
	File string

	// Line is the 1-based line number of the location of the
	// stack frame, or zero if unknown.
	Line int

	// Function is the name of the function name for this stack
	// frame. This should be package-qualified, and may be split
	// using stacktrace.SplitFunctionName.
	Function string
}
