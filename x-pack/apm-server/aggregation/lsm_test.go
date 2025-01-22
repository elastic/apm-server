package aggregation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRumAgent(t *testing.T) {
	cases := []struct {
		name   string
		expect bool
	}{
		{"rum-js", true},
		{"js-base", true},
		{"android/java", true},
		{"iOS/swift", true},
		{"nodejs", false},
		{"python", false},
		{"java", false},
		{"dotnet", false},
	}

	for _, tt := range cases {
		got := isRumAgent(tt.name)
		assert.Equal(t, tt.expect, got)
	}
}
