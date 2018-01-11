package sourcemap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmapIdKey(t *testing.T) {
	tests := []struct {
		id    Id
		key   string
		valid bool
	}{
		{
			id:    Id{ServiceName: "foo", ServiceVersion: "1.0", Path: "a/b"},
			key:   "foo_1.0_a/b",
			valid: true,
		},
		{
			id:    Id{ServiceName: "foo", ServiceVersion: "bar"},
			key:   "foo_bar",
			valid: false,
		},
		{id: Id{ServiceName: "foo"}, key: "foo", valid: false},
		{id: Id{ServiceVersion: "1"}, key: "1", valid: false},
		{id: Id{Path: "/tmp/a"}, key: "/tmp/a", valid: false},
	}
	for idx, test := range tests {
		assert.Equal(t, test.key, test.id.Key(),
			fmt.Sprintf("(%v) Expected Key() to return <%v> but received<%v>", idx, test.key, test.id.Key()))
		assert.Equal(t, test.valid, test.id.Valid(),
			fmt.Sprintf("(%v) Expected Valid() to be <%v>", idx, test.valid))
	}
}
