package k8s

import (
	"fmt"
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
