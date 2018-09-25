package model

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/internal/fastjson"
)

//go:generate go run ../internal/fastjson/generate.go -f -o marshal_fastjson.go .

const (
	// YYYY-MM-DDTHH:mm:ss.sssZ
	dateTimeFormat = "2006-01-02T15:04:05.999Z"
)

// MarshalFastJSON writes the JSON representation of t to w.
func (t Time) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('"')
	w.Time(time.Time(t), dateTimeFormat)
	w.RawByte('"')
}

// UnmarshalJSON unmarshals the JSON data into t.
func (t *Time) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	time, err := time.Parse(dateTimeFormat, s)
	if err != nil {
		return err
	}
	*t = Time(time)
	return nil
}

// UnmarshalJSON unmarshals the JSON data into v.
func (v *HTTPSpanContext) UnmarshalJSON(data []byte) error {
	var httpSpanContext struct {
		URL string
	}
	if err := json.Unmarshal(data, &httpSpanContext); err != nil {
		return err
	}
	u, err := url.Parse(httpSpanContext.URL)
	if err != nil {
		return err
	}
	v.URL = u
	return nil
}

// MarshalFastJSON writes the JSON representation of v to w.
func (v *HTTPSpanContext) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('{')
	beforeURL := w.Size()
	w.RawString(`"url":"`)
	if v.marshalURL(w) {
		w.RawByte('"')
	} else {
		w.Rewind(beforeURL)
	}
	w.RawByte('}')
}

func (v *HTTPSpanContext) marshalURL(w *fastjson.Writer) bool {
	if v.URL.Scheme != "" {
		if !marshalScheme(w, v.URL.Scheme) {
			return false
		}
		w.RawString("://")
	} else {
		w.RawString("http://")
	}
	w.StringContents(v.URL.Host)
	if v.URL.Path == "" {
		w.RawByte('/')
	} else {
		w.StringContents(v.URL.Path)
	}
	if v.URL.RawQuery != "" {
		w.RawByte('?')
		w.StringContents(v.URL.RawQuery)
	}
	if v.URL.Fragment != "" {
		w.RawByte('#')
		w.StringContents(v.URL.Fragment)
	}
	return true
}

// MarshalFastJSON writes the JSON representation of v to w.
func (v *URL) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('{')
	first := true
	if v.Hash != "" {
		const prefix = ",\"hash\":"
		if first {
			first = false
			w.RawString(prefix[1:])
		} else {
			w.RawString(prefix)
		}
		w.String(v.Hash)
	}
	if v.Hostname != "" {
		const prefix = ",\"hostname\":"
		if first {
			first = false
			w.RawString(prefix[1:])
		} else {
			w.RawString(prefix)
		}
		w.String(v.Hostname)
	}
	if v.Path != "" {
		const prefix = ",\"pathname\":"
		if first {
			first = false
			w.RawString(prefix[1:])
		} else {
			w.RawString(prefix)
		}
		w.String(v.Path)
	}
	if v.Port != "" {
		const prefix = ",\"port\":"
		if first {
			first = false
			w.RawString(prefix[1:])
		} else {
			w.RawString(prefix)
		}
		w.String(v.Port)
	}
	schemeBegin := -1
	schemeEnd := -1
	if v.Protocol != "" {
		before := w.Size()
		const prefix = ",\"protocol\":\""
		if first {
			first = false
			w.RawString(prefix[1:])
		} else {
			w.RawString(prefix)
		}
		schemeBegin = w.Size()
		if marshalScheme(w, v.Protocol) {
			schemeEnd = w.Size()
			w.RawByte('"')
		} else {
			w.Rewind(before)
		}
	}
	if v.Search != "" {
		const prefix = ",\"search\":"
		if first {
			first = false
			w.RawString(prefix[1:])
		} else {
			w.RawString(prefix)
		}
		w.String(v.Search)
	}
	if schemeEnd != -1 && v.Hostname != "" && v.Path != "" {
		before := w.Size()
		w.RawString(",\"full\":")
		if !v.marshalFullURL(w, w.Bytes()[schemeBegin:schemeEnd]) {
			w.Rewind(before)
		}
	}
	w.RawByte('}')
}

func marshalScheme(w *fastjson.Writer, scheme string) bool {
	// Canonicalize the scheme to lowercase. Don't use
	// strings.ToLower, as it's too general and requires
	// additional memory allocations.
	//
	// The scheme should start with a letter, and may
	// then be followed by letters, digits, '+', '-',
	// and '.'. We don't validate the scheme here, we
	// just use those restrictions as a basis for
	// optimization; anything not in that set will
	// mean the full URL is omitted.
	for i := 0; i < len(scheme); i++ {
		c := scheme[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.':
			w.RawByte(c)
		case c >= 'A' && c <= 'Z':
			w.RawByte(c + 'a' - 'A')
		default:
			return false
		}
	}
	return true
}

func (v *URL) marshalFullURL(w *fastjson.Writer, scheme []byte) bool {
	w.RawByte('"')
	before := w.Size()
	w.RawBytes(scheme)
	w.RawString("://")
	if strings.IndexByte(v.Hostname, ':') == -1 {
		w.StringContents(v.Hostname)
	} else {
		w.RawByte('[')
		w.StringContents(v.Hostname)
		w.RawByte(']')
	}
	if v.Port != "" {
		w.RawByte(':')
		w.StringContents(v.Port)
	}
	w.StringContents(v.Path)
	if v.Search != "" {
		w.RawByte('?')
		w.StringContents(v.Search)
	}
	if v.Hash != "" {
		w.RawByte('#')
		w.StringContents(v.Hash)
	}
	if n := w.Size() - before; n > 1024 {
		// Truncate the full URL to 1024 bytes.
		w.Rewind(w.Size() - n + 1024)
	}
	w.RawByte('"')
	return true
}

func (c *SpanCount) isZero() bool {
	return *c == SpanCount{}
}

func (d *SpanCountDropped) isZero() bool {
	return *d == SpanCountDropped{}
}

func (l *Log) isZero() bool {
	return l.Message == ""
}

func (e *Exception) isZero() bool {
	return e.Message == ""
}

func (c Cookies) isZero() bool {
	return len(c) == 0
}

// MarshalFastJSON writes the JSON representation of c to w.
func (c Cookies) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('{')
	first := true
outer:
	for i := len(c) - 1; i >= 0; i-- {
		for j := i + 1; j < len(c); j++ {
			if c[i].Name == c[j].Name {
				continue outer
			}
		}
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(c[i].Name)
		w.RawByte(':')
		w.String(c[i].Value)
	}
	w.RawByte('}')
}

// UnmarshalJSON unmarshals the JSON data into c.
func (c *Cookies) UnmarshalJSON(data []byte) error {
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*c = make([]*http.Cookie, 0, len(m))
	for k, v := range m {
		*c = append(*c, &http.Cookie{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].Name < (*c)[j].Name
	})
	return nil
}

// isZero is used by fastjson to implement omitempty.
func (t *TransactionReference) isZero() bool {
	return t.ID.isZero()
}

// MarshalFastJSON writes the JSON representation of c to w.
func (c *ExceptionCode) MarshalFastJSON(w *fastjson.Writer) {
	if c.String != "" {
		w.String(c.String)
		return
	}
	w.Float64(c.Number)
}

// UnmarshalJSON unmarshals the JSON data into c.
func (c *ExceptionCode) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case string:
		c.String = v
	case float64:
		c.Number = v
	default:
		return errors.Errorf("expected string or number, got %T", v)
	}
	return nil
}

// isZero is used by fastjson to implement omitempty.
func (c *ExceptionCode) isZero() bool {
	return c.String == "" && c.Number == 0
}

// MarshalFastJSON writes the JSON representation of b to w.
func (b *RequestBody) MarshalFastJSON(w *fastjson.Writer) {
	if b.Form != nil {
		w.RawByte('{')
		first := true
		for k, v := range b.Form {
			if first {
				first = false
			} else {
				w.RawByte(',')
			}
			w.String(k)
			w.RawByte(':')
			if len(v) == 1 {
				// Just one item, add the item directly.
				w.String(v[0])
			} else {
				// Zero or multiple items, include them all.
				w.RawByte('[')
				first := true
				for _, v := range v {
					if first {
						first = false
					} else {
						w.RawByte(',')
					}
					w.String(v)
				}
				w.RawByte(']')
			}
		}
		w.RawByte('}')
	} else {
		w.String(b.Raw)
	}
}

// UnmarshalJSON unmarshals the JSON data into b.
func (b *RequestBody) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case string:
		b.Raw = v
		return nil
	case map[string]interface{}:
		form := make(url.Values, len(v))
		for k, v := range v {
			switch v := v.(type) {
			case string:
				form.Set(k, v)
			case []interface{}:
				for _, v := range v {
					switch v := v.(type) {
					case string:
						form.Add(k, v)
					default:
						return errors.Errorf("expected string, got %T", v)
					}
				}
			default:
				return errors.Errorf("expected string or []string, got %T", v)
			}
		}
		b.Form = form
	default:
		return errors.Errorf("expected string or map, got %T", v)
	}
	return nil
}

func (m IfaceMap) isZero() bool {
	return len(m) == 0
}

// MarshalFastJSON writes the JSON representation of m to w.
func (m IfaceMap) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('{')
	first := true
	for _, item := range m {
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(item.Key)
		w.RawByte(':')
		fastjson.Marshal(w, item.Value)
	}
	w.RawByte('}')
}

// UnmarshalJSON unmarshals the JSON data into m.
func (m *IfaceMap) UnmarshalJSON(data []byte) error {
	var mm map[string]interface{}
	if err := json.Unmarshal(data, &mm); err != nil {
		return err
	}
	*m = make(IfaceMap, 0, len(mm))
	for k, v := range mm {
		*m = append(*m, IfaceMapItem{Key: k, Value: v})
	}
	sort.Slice(*m, func(i, j int) bool {
		return (*m)[i].Key < (*m)[j].Key
	})
	return nil
}

// MarshalFastJSON exists to prevent code generation for IfaceMapItem.
func (*IfaceMapItem) MarshalFastJSON(*fastjson.Writer) {
	panic("unreachable")
}

func (m StringMap) isZero() bool {
	return len(m) == 0
}

// MarshalFastJSON writes the JSON representation of m to w.
func (m StringMap) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('{')
	first := true
	for _, item := range m {
		if first {
			first = false
		} else {
			w.RawByte(',')
		}
		w.String(item.Key)
		w.RawByte(':')
		fastjson.Marshal(w, item.Value)
	}
	w.RawByte('}')
}

// UnmarshalJSON unmarshals the JSON data into m.
func (m *StringMap) UnmarshalJSON(data []byte) error {
	var mm map[string]string
	if err := json.Unmarshal(data, &mm); err != nil {
		return err
	}
	*m = make(StringMap, 0, len(mm))
	for k, v := range mm {
		*m = append(*m, StringMapItem{Key: k, Value: v})
	}
	sort.Slice(*m, func(i, j int) bool {
		return (*m)[i].Key < (*m)[j].Key
	})
	return nil
}

// MarshalFastJSON exists to prevent code generation for StringMapItem.
func (*StringMapItem) MarshalFastJSON(*fastjson.Writer) {
	panic("unreachable")
}

// MarshalFastJSON writes the JSON representation of id to w.
func (id *TransactionID) MarshalFastJSON(w *fastjson.Writer) {
	if !id.SpanID.isZero() {
		id.SpanID.MarshalFastJSON(w)
		return
	}
	id.UUID.MarshalFastJSON(w)
}

// UnmarshalJSON unmarshals the JSON data into id.
func (id *TransactionID) UnmarshalJSON(data []byte) error {
	if len(data) == len(id.SpanID)*2+2 {
		return id.SpanID.UnmarshalJSON(data)
	}
	return id.UUID.UnmarshalJSON(data)
}

func (id *TraceID) isZero() bool {
	return *id == TraceID{}
}

// MarshalFastJSON writes the JSON representation of id to w.
func (id *TraceID) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('"')
	writeHex(w, id[:])
	w.RawByte('"')
}

// UnmarshalJSON unmarshals the JSON data into id.
func (id *TraceID) UnmarshalJSON(data []byte) error {
	_, err := hex.Decode(id[:], data[1:len(data)-1])
	return err
}

func (id *SpanID) isZero() bool {
	return *id == SpanID{}
}

// UnmarshalJSON unmarshals the JSON data into id.
func (id *SpanID) UnmarshalJSON(data []byte) error {
	_, err := hex.Decode(id[:], data[1:len(data)-1])
	return err
}

// MarshalFastJSON writes the JSON representation of id to w.
func (id *SpanID) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('"')
	writeHex(w, id[:])
	w.RawByte('"')
}

func (id *UUID) isZero() bool {
	return *id == UUID{}
}

// UnmarshalJSON unmarshals the JSON data into id.
func (id *UUID) UnmarshalJSON(data []byte) error {
	// NOTE(axw) UnmarshalJSON is provided only for tests;
	// it should only ever be fed valid data, hence panics.
	hexDecode := func(out, in []byte) {
		if _, err := hex.Decode(out, in); err != nil {
			panic(err)
		}
	}
	data = data[1 : len(data)-1]
	hexDecode(id[:4], data[:8])
	hexDecode(id[4:6], data[9:13])
	hexDecode(id[6:8], data[14:18])
	hexDecode(id[8:10], data[19:23])
	hexDecode(id[10:], data[24:])
	return nil
}

// MarshalFastJSON writes the JSON representation of id to w.
func (id *UUID) MarshalFastJSON(w *fastjson.Writer) {
	w.RawByte('"')
	writeHex(w, id[:4])
	w.RawByte('-')
	writeHex(w, id[4:6])
	w.RawByte('-')
	writeHex(w, id[6:8])
	w.RawByte('-')
	writeHex(w, id[8:10])
	w.RawByte('-')
	writeHex(w, id[10:])
	w.RawByte('"')
}

func writeHex(w *fastjson.Writer, v []byte) {
	const hextable = "0123456789abcdef"
	for _, v := range v {
		w.RawByte(hextable[v>>4])
		w.RawByte(hextable[v&0x0f])
	}
}
