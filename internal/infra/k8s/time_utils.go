package k8s

import (
	"fmt"
	"strconv"
	"time"
)

// FormatAge returns the duration in s/m/h/d with max-granularity.
func FormatAge(d time.Duration) string {
	switch {
	case d.Hours() >= 48:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	case d.Hours() >= 1:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d.Minutes() >= 1:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

// ParseAge converts strings like "10s", "3m", "4h", "2d" to seconds.
// Unsupported or malformed inputs return 0.
func ParseAge(s string) int64 {
	if len(s) < 2 {
		return 0
	}
	unit := s[len(s)-1]
	var mult int64
	switch unit {
	case 's':
		mult = 1
	case 'm':
		mult = 60
	case 'h':
		mult = 3600
	case 'd':
		mult = 86400
	default:
		return 0
	}
	v, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v * mult
}
