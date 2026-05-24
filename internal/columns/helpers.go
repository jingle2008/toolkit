package columns

import (
	"fmt"

	"github.com/jingle2008/toolkit/pkg/models"
)

// limitOverrideMin returns Values[0].Min as a string, or "" when
// Values is empty (avoids the index-out-of-range that the current
// limitTenancyOverrideToRow assumes-away).
func limitOverrideMin(values []models.LimitRange) string {
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprint(values[0].Min)
}

// limitOverrideMax returns Values[0].Max as a string, or "" when
// Values is empty.
func limitOverrideMax(values []models.LimitRange) string {
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprint(values[0].Max)
}
