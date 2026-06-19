// Package telemetry builds OCI Telemetry MQL Explore dashboard URLs.
package telemetry

import "strings"

// base62Alphabet is Zipson's integer alphabet.
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// base62 encodes n using Zipson's base62 alphabet. Negative values are
// prefixed with '-'. Used for Zipson integer tokens (e.g. epoch-ms
// timestamps).
func base62(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = base62Alphabet[n%62]
		n /= 62
	}
	s := string(buf[i:])
	if neg {
		return "-" + s
	}
	return s
}

// Zipson serialization tokens (see OCI Telemetry MQL Decode notes).
const (
	tokenString = "¨" // ¨  string delimiter
	tokenInt    = "¢" // ¢  integer token
	tokenTrue   = "»" // »  boolean true
	tokenFalse  = "«" // «  boolean false
	tokenArrEnd = "÷" // ÷  array end
)

// Encoder builds a Zipson payload. Strings are emitted in full (no
// reference compression), which is still valid Zipson and decodes
// correctly. It does NOT escape the string-delimiter rune (U+00A8); the
// caller must not pass values containing it.
type Encoder struct {
	b strings.Builder
}

// BeginObject writes the object-start token '{'.
func (e *Encoder) BeginObject() *Encoder { e.b.WriteByte('{'); return e }

// EndObject writes the object-end token '}'.
func (e *Encoder) EndObject() *Encoder { e.b.WriteByte('}'); return e }

// BeginArray writes the array-start token '|'.
func (e *Encoder) BeginArray() *Encoder { e.b.WriteByte('|'); return e }

// EndArray writes the array-end token '÷'.
func (e *Encoder) EndArray() *Encoder { e.b.WriteString(tokenArrEnd); return e }

// Str writes a delimited Zipson string.
func (e *Encoder) Str(s string) *Encoder {
	e.b.WriteString(tokenString)
	e.b.WriteString(s)
	e.b.WriteString(tokenString)
	return e
}

// Key writes an object key (same wire form as a string).
func (e *Encoder) Key(s string) *Encoder { return e.Str(s) }

// Int writes a Zipson integer token (¢ + base62).
func (e *Encoder) Int(n int64) *Encoder {
	e.b.WriteString(tokenInt)
	e.b.WriteString(base62(n))
	return e
}

// Bool writes a Zipson boolean token.
func (e *Encoder) Bool(v bool) *Encoder {
	if v {
		e.b.WriteString(tokenTrue)
	} else {
		e.b.WriteString(tokenFalse)
	}
	return e
}

// String returns the serialized Zipson payload.
func (e *Encoder) String() string { return e.b.String() }
