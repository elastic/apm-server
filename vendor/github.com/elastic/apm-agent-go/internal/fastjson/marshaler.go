package fastjson

import (
	"encoding/json"
	"fmt"
)

// Marshaler defines an interface that types can implement to provide
// fast JSON marshaling.
type Marshaler interface {
	// MarshalFastJSON writes a JSON representation of the type to w.
	MarshalFastJSON(w *Writer)
}

// Appender defines an interface that types can implement to append
// their JSON representation to a buffer. If the value is not a valid
// JSON token, it will be rejected.
type Appender interface {
	// AppendJSON appends the JSON representation of the value to the
	// buffer, and returns the extended buffer.
	AppendJSON([]byte) []byte
}

// Marshal marshals v as JSON to w.
func Marshal(w *Writer, v interface{}) {
	switch v := v.(type) {
	case nil:
		w.RawString("null")
	case string:
		w.String(v)
	case uint:
		w.Uint64(uint64(v))
	case uint8:
		w.Uint64(uint64(v))
	case uint16:
		w.Uint64(uint64(v))
	case uint32:
		w.Uint64(uint64(v))
	case uint64:
		w.Uint64(v)
	case int:
		w.Int64(int64(v))
	case int8:
		w.Int64(int64(v))
	case int16:
		w.Int64(int64(v))
	case int32:
		w.Int64(int64(v))
	case int64:
		w.Int64(v)
	case float32:
		w.Float32(v)
	case float64:
		w.Float64(v)
	case bool:
		w.Bool(v)
	case map[string]interface{}:
		if v == nil {
			w.RawString("null")
			return
		}
		w.RawByte('{')
		first := true
		for k, v := range v {
			if first {
				first = false
			} else {
				w.RawByte(',')
			}
			w.String(k)
			w.RawByte(':')
			Marshal(w, v)
		}
		w.RawByte('}')
	case Marshaler:
		v.MarshalFastJSON(w)
	case Appender:
		w.buf = v.AppendJSON(w.buf)
	default:
		marshalReflect(w, v)
	}
}

func marshalReflect(w *Writer, v interface{}) {
	defer func() {
		if r := recover(); r != nil {
			w.RawString(`{"__PANIC__":`)
			w.String(fmt.Sprintf("panic calling MarshalJSON for type %T: %v", v, r))
			w.RawByte('}')
		}
	}()
	raw, err := json.Marshal(v)
	if err != nil {
		w.RawString(`{"__ERROR__":`)
		w.String(fmt.Sprint(err))
		w.RawByte('}')
		return
	}
	w.RawBytes(raw)
}
