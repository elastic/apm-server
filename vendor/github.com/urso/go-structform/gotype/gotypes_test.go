package gotype

import (
	"testing"

	structform "github.com/urso/go-structform"
	"github.com/urso/go-structform/sftest"
)

type mapstr map[string]interface{}

func TestFoldUnfoldToIfcConsistent(t *testing.T) {
	sftest.TestEncodeParseConsistent(t, sftest.Samples,
		func() (structform.Visitor, func(structform.Visitor) error) {
			var v interface{}
			unfolder, err := NewUnfolder(&v)
			if err != nil {
				panic(err)
			}
			return unfolder, func(to structform.Visitor) error {
				return Fold(v, to)
			}
		})
}
