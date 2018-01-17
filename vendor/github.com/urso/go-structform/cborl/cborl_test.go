package cborl

import (
	"bytes"
	"testing"

	structform "github.com/urso/go-structform"
	"github.com/urso/go-structform/sftest"
)

func TestEncParseConsistent(t *testing.T) {
	testEncParseConsistent(t, Parse)
}

func TestEncDecoderConsistent(t *testing.T) {
	testEncParseConsistent(t, func(content []byte, to structform.Visitor) error {
		dec := NewBytesDecoder(content, to)
		return dec.Next()
	})
}

func TestEncParseBytesConsistent(t *testing.T) {
	testEncParseConsistent(t, func(content []byte, to structform.Visitor) error {
		p := NewParser(to)
		for _, b := range content {
			err := p.feed([]byte{b})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func testEncParseConsistent(
	t *testing.T,
	parse func([]byte, structform.Visitor) error,
) {
	sftest.TestEncodeParseConsistent(t, sftest.Samples,
		func() (structform.Visitor, func(structform.Visitor) error) {
			buf := bytes.NewBuffer(nil)
			vs := NewVisitor(buf)

			return vs, func(to structform.Visitor) error {
				return parse(buf.Bytes(), to)
			}
		})
}
