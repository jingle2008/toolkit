package cli

import "time"

// daysAgo is a creation time n days (and an hour) ago, so its age renders
// as "<n>d" — or, under two days, in hours ("25h" for 1). Whole seconds
// in UTC, as the API server writes them.
func daysAgo(n int) time.Time {
	return time.Now().Add(-time.Duration(n)*24*time.Hour - time.Hour).UTC().Truncate(time.Second)
}
