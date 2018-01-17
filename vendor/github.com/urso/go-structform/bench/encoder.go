package bench

import (
	"bytes"
	stdjson "encoding/json"
	"io"

	// jsoniter "github.com/json-iterator/go"
	"github.com/ugorji/go/codec"
	"github.com/urso/go-structform/cborl"
	"github.com/urso/go-structform/gotype"
	"github.com/urso/go-structform/json"
	"github.com/urso/go-structform/ubjson"
)

type encoderFactory func(io.Writer) func(interface{}) error
type decoderFactory func([]byte) func(interface{}) error
type transcodeFactory func(io.Writer) func([]byte) error

func stdJSONEncoder(w io.Writer) func(interface{}) error {
	enc := stdjson.NewEncoder(w)
	return enc.Encode
}

func stdJSONDecoder(r io.Reader) func(interface{}) error {
	dec := stdjson.NewDecoder(r)
	return dec.Decode
}

func stdJSONBufDecoder(b []byte) func(interface{}) error {
	return stdJSONDecoder(bytes.NewReader(b))
}

func gocodecJSONDecoder(r io.Reader) func(interface{}) error {
	h := &codec.JsonHandle{}
	dec := codec.NewDecoder(r, h)
	return dec.Decode
}

/*
func jsoniterDecoder(r io.Reader) func(interface{}) error {
	iter := jsoniter.Parse(r, 4096)
	return func(v interface{}) error {
		iter.ReadVal(v)
		return iter.Error
	}
}

func jsoniterBufDecoder(b []byte) func(interface{}) error {
	iter := jsoniter.ParseBytes(b)
	return func(v interface{}) error {
		iter.ReadVal(v)
		return iter.Error
	}
}
*/

func structformJSONEncoder(w io.Writer) func(interface{}) error {
	vs := json.NewVisitor(w)
	folder, _ := gotype.NewIterator(vs)
	return folder.Fold
}

func structformUBJSONEncoder(w io.Writer) func(interface{}) error {
	vs := ubjson.NewVisitor(w)
	folder, _ := gotype.NewIterator(vs)
	return folder.Fold
}

func structformCBORLEncoder(w io.Writer) func(interface{}) error {
	vs := cborl.NewVisitor(w)
	folder, _ := gotype.NewIterator(vs)
	return folder.Fold
}

func structformJSONBufDecoder(keyCache int) func([]byte) func(interface{}) error {
	return func(b []byte) func(interface{}) error {
		u, _ := gotype.NewUnfolder(nil)
		dec := json.NewBytesDecoder(b, u)
		return makeStructformDecoder(u, dec.Next, keyCache)
	}
}

func structformUBJSONBufDecoder(keyCache int) func([]byte) func(interface{}) error {
	return func(b []byte) func(interface{}) error {
		u, _ := gotype.NewUnfolder(nil)
		dec := ubjson.NewBytesDecoder(b, u)
		return makeStructformDecoder(u, dec.Next, keyCache)
	}
}

func structformCBORLBufDecoder(keyCache int) func([]byte) func(interface{}) error {
	return func(b []byte) func(interface{}) error {
		u, _ := gotype.NewUnfolder(nil)
		dec := cborl.NewBytesDecoder(b, u)
		return makeStructformDecoder(u, dec.Next, keyCache)
	}
}

func makeStructformDecoder(
	u *gotype.Unfolder,
	next func() error,
	keyCache int,
) func(interface{}) error {
	if keyCache > 0 {
		u.EnableKeyCache(keyCache)
	}
	return func(v interface{}) error {
		if err := u.SetTarget(v); err != nil {
			return err
		}
		return next()
	}
}

func makeCBORL2JSONTranscoder(w io.Writer) func([]byte) error {
	j := json.NewVisitor(w)
	p := cborl.NewParser(j)
	return p.Parse
}

func makeUBJSON2JSONTranscoder(w io.Writer) func([]byte) error {
	j := json.NewVisitor(w)
	p := ubjson.NewParser(j)
	return p.Parse
}
