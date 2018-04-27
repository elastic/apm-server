package stacktrace_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/stacktrace"
)

func TestLibraryPackage(t *testing.T) {
	assert.True(t, stacktrace.IsLibraryPackage("encoding/json"))
	assert.True(t, stacktrace.IsLibraryPackage("encoding/json/zzz"))
	assert.False(t, stacktrace.IsLibraryPackage("encoding/jsonzzz"))

	stacktrace.RegisterLibraryPackage("encoding/jsonzzz")
	assert.True(t, stacktrace.IsLibraryPackage("encoding/jsonzzz"))
	assert.True(t, stacktrace.IsLibraryPackage("encoding/jsonzzz/yyy"))

	stacktrace.RegisterApplicationPackage("encoding/jsonzzz/yyy")
	assert.True(t, stacktrace.IsLibraryPackage("encoding/jsonzzz"))
	assert.False(t, stacktrace.IsLibraryPackage("encoding/jsonzzz/yyy"))
	assert.False(t, stacktrace.IsLibraryPackage("encoding/jsonzzz/yyy/xxx"))
}
