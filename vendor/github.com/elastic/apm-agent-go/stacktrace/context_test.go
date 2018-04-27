package stacktrace_test

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

func TestFilesystemContextSetter(t *testing.T) {
	setter := stacktrace.FileSystemContextSetter(http.Dir("./testdata"))
	frame := model.StacktraceFrame{
		AbsolutePath: "/foo.go",
		Line:         5,
	}

	data, err := ioutil.ReadFile("./testdata/foo.go")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	testSetContext(t, setter, frame, 2, 1,
		lines[4],
		lines[2:4],
		lines[5:],
	)
	testSetContext(t, setter, frame, 0, 0, lines[4], []string{}, []string{})
	testSetContext(t, setter, frame, 500, 0, lines[4], lines[:4], []string{})
	testSetContext(t, setter, frame, 0, 500, lines[4], []string{}, lines[5:])
}

func TestFilesystemContextSetterFileNotFound(t *testing.T) {
	setter := stacktrace.FileSystemContextSetter(http.Dir("./testdata"))
	frame := model.StacktraceFrame{
		AbsolutePath: "/foo.go",
		Line:         5,
	}

	data, err := ioutil.ReadFile("./testdata/foo.go")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	testSetContext(t, setter, frame, 2, 1,
		lines[4],
		lines[2:4],
		lines[5:],
	)
	testSetContext(t, setter, frame, 0, 0, lines[4], []string{}, []string{})
	testSetContext(t, setter, frame, 500, 0, lines[4], lines[:4], []string{})
	testSetContext(t, setter, frame, 0, 500, lines[4], []string{}, lines[5:])
}

func testSetContext(
	t *testing.T,
	setter stacktrace.ContextSetter,
	frame model.StacktraceFrame,
	nPre, nPost int,
	expectedContext string, expectedPre, expectedPost []string,
) {
	frames := []model.StacktraceFrame{frame}
	err := stacktrace.SetContext(setter, frames, nPre, nPost)
	if err != nil {
		t.Fatalf("SetContext failed: %s", err)
	}
	if diff := cmp.Diff(frames[0].ContextLine, expectedContext); diff != "" {
		t.Fatalf("ContextLine differs: %s", diff)
	}
	if diff := cmp.Diff(frames[0].PreContext, expectedPre); diff != "" {
		t.Fatalf("PreContext differs: %s", diff)
	}
	if diff := cmp.Diff(frames[0].PostContext, expectedPost); diff != "" {
		t.Fatalf("PostContext differs: %s", diff)
	}
}
