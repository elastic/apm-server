package tests

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type group struct {
	str string
}

func Group(s string) group {
	return group{str: s}
}

func strConcat(pre string, post string, delimiter string) string {
	if pre == "" {
		return post
	}
	return pre + delimiter + post
}

func differenceWithGroup(s1 *Set, s2 *Set) *Set {
	s := Difference(s1, s2)

	for _, e2 := range s2.Array() {
		if e2Grp, ok := e2.(group); !ok {
			continue
		} else {
			for _, e1 := range s1.Array() {
				if e1Str, ok := e1.(string); ok {
					if strings.HasPrefix(e1Str, e2Grp.str) {
						s.Remove(e1)
					}
				}
			}

		}
	}
	return s
}

func assertEmptySet(t *testing.T, s *Set, msg string) {
	if s.Len() > 0 {
		assert.Fail(t, msg)
	}
}
