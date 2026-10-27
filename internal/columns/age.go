package columns

import (
	"time"

	"github.com/jingle2008/toolkit/pkg/models"
)

// freshAge is the time since created as of now, so a row rebuilt from a
// cached load shows its current age; stored, the age as of the load, is
// for a row that carries no creation time.
func freshAge(created time.Time, stored string) string {
	if created.IsZero() {
		return stored
	}
	return models.AgeOf(created)
}
