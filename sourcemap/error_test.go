package sourcemap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	tests := []struct {
		Msg     string
		Kind    Enum
		OutMsg  string
		OutKind string
	}{
		{OutMsg: "", OutKind: ""},
		{Msg: "Init failed", Kind: InitError, OutMsg: "Init failed", OutKind: "InitError"},
		{Msg: "Access failed", Kind: AccessError, OutMsg: "Access failed", OutKind: "AccessError"},
		{Msg: "Map failed", Kind: MapError, OutMsg: "Map failed", OutKind: "MapError"},
		{Msg: "Parse failed", Kind: ParseError, OutMsg: "Parse failed", OutKind: "ParseError"},
		{Msg: "Key error", Kind: KeyError, OutMsg: "Key error", OutKind: "KeyError"},
	}
	for idx, test := range tests {
		err := Error{Msg: test.Msg, Kind: test.Kind}
		assert.Equal(t, test.OutMsg, err.Msg,
			fmt.Sprintf("(%v): Expected <%v>, Received <%v> ", idx, test.OutMsg, err.Msg))
		assert.Equal(t, test.OutKind, string(err.Kind),
			fmt.Sprintf("(%v): Expected <%v>, Received <%v> ", idx, test.OutKind, err.Kind))
	}
}
