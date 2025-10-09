package integrationservertest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

// TestInternal_ConfigLazyRollover will not be run in CI, make sure to run this test if
// you make any changes to the config parser.
func TestInternal_ConfigLazyRollover(t *testing.T) {
	const testConfig = `
lazy-rollover-exceptions:
  # (1) No lazy rollover from all versions 1.* to all versions 2.*.
  - from: "1.*.*"
    to:   "2.*.*"
  # (2) No lazy rollover from versions 3.0.* to versions between 3.1.0 and 3.1.5.
  - from: "3.0.*"
    to:   "[3.1.0 - 3.1.5]"
  # (3) No lazy rollover from versions 4.2.* >= 4.2.11 to versions 4.2.* >= 4.2.11.
  - from: "[4.2.11 - 4.3.0)"
    to:   "[4.2.11 - 4.3.0)"
  # (4) No lazy rollover from versions 5.* to versions 5.* iff same minor.
  - from: "5.x.*"
    to:   "5.x.*"
  # (5) No lazy rollover from versions 6.* between 6.2.4 and 6.4.8 to versions 6.* after 6.3.12.
  - from: "[6.2.4 - 6.4.8]"
    to:   "[6.3.12 - 7.0.0)"
  # (6) No lazy rollover from version 7.1.1 to version 7.1.2.
  - from: "7.1.1"
    to:   "7.1.2"
`

	cfg, err := parseConfig(strings.NewReader(testConfig))
	require.NoError(t, err)

	type args struct {
		from string
		to   string
	}
	tests := []struct {
		args args
		want bool
	}{
		// Case 1
		{
			args: args{
				from: "1.2.3",
				to:   "2.4.6",
			},
			want: false,
		},
		// Case 2
		{
			args: args{
				from: "3.0.15",
				to:   "3.1.2",
			},
			want: false,
		},
		{
			args: args{
				from: "3.0.6",
				to:   "3.1.12",
			},
			want: true,
		},
		// Case 3
		{
			args: args{
				from: "4.2.12",
				to:   "4.2.27",
			},
			want: false,
		},
		{
			args: args{
				from: "4.2.11",
				to:   "4.3.0",
			},
			want: true,
		},
		// Case 4
		{
			args: args{
				from: "5.1.2",
				to:   "5.1.6",
			},
			want: false,
		},
		{
			args: args{
				from: "5.1.2",
				to:   "5.8.3",
			},
			want: true,
		},
		// Case 5
		{
			args: args{
				from: "6.2.4",
				to:   "6.3.12",
			},
			want: false,
		},
		{
			args: args{
				from: "6.4.8",
				to:   "6.9.9",
			},
			want: false,
		},
		// Case 6
		{
			args: args{
				from: "7.1.1",
				to:   "7.1.2",
			},
			want: false,
		},
		{
			args: args{
				from: "7.1.1",
				to:   "7.2.3",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%s to %s", tt.args.from, tt.args.to)
		from, err := ech.NewVersionFromString(tt.args.from)
		require.NoError(t, err)
		to, err := ech.NewVersionFromString(tt.args.to)
		require.NoError(t, err)
		t.Run(name, func(t *testing.T) {
			if got := cfg.LazyRollover(from, to); got != tt.want {
				t.Errorf("LazyRollover() = %v, want %v", got, tt.want)
			}
		})
	}
}
