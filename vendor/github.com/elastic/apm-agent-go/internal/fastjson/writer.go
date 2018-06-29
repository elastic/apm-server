package fastjson

import (
	"strconv"
	"time"
	"unicode/utf8"
)

// Writer is a JSON writer.
type Writer struct {
	buf []byte
}

// Bytes returns the internal buffer. The result
// is invalidated when Reset is called.
func (w *Writer) Bytes() []byte {
	return w.buf
}

// Size returns the current size of the buffer.
func (w *Writer) Size() int {
	return len(w.buf)
}

// Rewind rewinds the buffer such that it has size bytes,
// dropping everything proceeding.
func (w *Writer) Rewind(size int) {
	w.buf = w.buf[:size]
}

// Reset resets the internal []byte buffer to empty.
func (w *Writer) Reset() {
	w.buf = w.buf[:0]
}

// RawByte appends c to the buffer.
func (w *Writer) RawByte(c byte) {
	w.buf = append(w.buf, c)
}

// RawBytes appends data, unmodified, to the buffer.
func (w *Writer) RawBytes(data []byte) {
	w.buf = append(w.buf, data...)
}

// RawString appends s to the buffer.
func (w *Writer) RawString(s string) {
	w.buf = append(w.buf, s...)
}

// Uint64 appends n to the buffer.
func (w *Writer) Uint64(n uint64) {
	w.buf = strconv.AppendUint(w.buf, uint64(n), 10)
}

// Int64 appends n to the buffer.
func (w *Writer) Int64(n int64) {
	w.buf = strconv.AppendInt(w.buf, int64(n), 10)
}

// Float32 appends n to the buffer.
func (w *Writer) Float32(n float32) {
	w.buf = strconv.AppendFloat(w.buf, float64(n), 'g', -1, 32)
}

// Float64 appends n to the buffer.
func (w *Writer) Float64(n float64) {
	w.buf = strconv.AppendFloat(w.buf, float64(n), 'g', -1, 64)
}

// Bool appends v to the buffer.
func (w *Writer) Bool(v bool) {
	w.buf = strconv.AppendBool(w.buf, v)
}

// Time appends t to the buffer, formatted according to layout.
func (w *Writer) Time(t time.Time, layout string) {
	w.buf = t.AppendFormat(w.buf, layout)
}

const chars = "0123456789abcdef"

func isNotEscapedSingleChar(c byte, escapeHTML bool) bool {
	// Note: might make sense to use a table if there are more chars to escape. With 4 chars
	// it benchmarks the same.
	if escapeHTML {
		return c != '<' && c != '>' && c != '&' && c != '\\' && c != '"' && c >= 0x20 && c < utf8.RuneSelf
	}
	return c != '\\' && c != '"' && c >= 0x20 && c < utf8.RuneSelf
}

// String appends s, quoted and escaped, to the buffer.
func (w *Writer) String(s string) {
	w.RawByte('"')
	w.StringContents(s)
	w.RawByte('"')
}

// StringContents is the same as String, but without the surrounding quotes.
func (w *Writer) StringContents(s string) {
	// Portions of the string that contain no escapes are appended as
	// byte slices.

	p := 0 // last non-escape symbol

	for i := 0; i < len(s); {
		c := s[i]

		if isNotEscapedSingleChar(c, true) {
			// single-width character, no escaping is required
			i++
			continue
		} else if c < utf8.RuneSelf {
			// single-with character, need to escape
			w.RawString(s[p:i])
			switch c {
			case '\t':
				w.RawString(`\t`)
			case '\r':
				w.RawString(`\r`)
			case '\n':
				w.RawString(`\n`)
			case '\\':
				w.RawString(`\\`)
			case '"':
				w.RawString(`\"`)
			default:
				w.RawString(`\u00`)
				w.RawByte(chars[c>>4])
				w.RawByte(chars[c&0xf])
			}

			i++
			p = i
			continue
		}

		// broken utf
		runeValue, runeWidth := utf8.DecodeRuneInString(s[i:])
		if runeValue == utf8.RuneError && runeWidth == 1 {
			w.RawString(s[p:i])
			w.RawString(`\ufffd`)
			i++
			p = i
			continue
		}

		// jsonp stuff - tab separator and line separator
		if runeValue == '\u2028' || runeValue == '\u2029' {
			w.RawString(s[p:i])
			w.RawString(`\u202`)
			w.RawByte(chars[runeValue&0xf])
			i += runeWidth
			p = i
			continue
		}
		i += runeWidth
	}
	w.RawString(s[p:])
}
