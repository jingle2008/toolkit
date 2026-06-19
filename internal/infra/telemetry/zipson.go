// Package telemetry builds OCI Telemetry MQL Explore dashboard URLs.
package telemetry

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
