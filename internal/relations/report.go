package relations

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jingle2008/toolkit/internal/domain"
)

// FaultyCategoryErr reports, for `get --faulty` and list_related's
// faulty, that category's rows can't be faulty; nil when they can.
func FaultyCategoryErr(category domain.Category) error {
	if IsFaultyCategory(category) {
		return nil
	}
	names := make([]string, 0, len(faultyCategories))
	for _, c := range faultyCategories {
		names = append(names, strings.ToLower(c.String()))
	}
	slices.Sort(names)
	return fmt.Errorf("%s rows are never faulty; faulty applies to: %s", category, strings.Join(names, ", "))
}

// Hint says how to fix c's dangling rows, "" when there are none: doctor
// --relations' HINT and check_integrity's hint.
func (c Check) Hint() string {
	if len(c.Dangling) == 0 {
		return ""
	}
	if c.Parent == domain.Tenant {
		return tenancyHint(c.Dangling)
	}
	return fmt.Sprintf("these %s rows name a %s that doesn't exist: clean them up or restore the %s", c.Child, c.Parent, c.Parent)
}

// tenancyHint is the tenancy check's remedy: tenancies missing from the
// metadata are added there, while an orphan has no tenancy to add (the
// TUI won't edit its tenant) and needs its CR labelled.
func tenancyHint(dangling []Dangling) string {
	var unknown, orphans bool
	for _, d := range dangling {
		if d.Missing == MissingTenancyLabel {
			orphans = true
		} else {
			unknown = true
		}
	}
	var hints []string
	if unknown {
		hints = append(hints, "add the tenancies to the metadata file (TUI: edit tenant on an unresolved DAC/ImportedModel)")
	}
	if orphans {
		hints = append(hints, "set the tenancy-id label on the CRs missing it")
	}
	return strings.Join(hints, "; ")
}

// NotePartialPools adds poolNote — which GPU pool sources failed to load
// — to the GPUNode → GPUPool check when it reports missing pools: such a
// pool may just be unloaded. The check stays FAIL, as the TUI flags those
// nodes: a broken source needs fixing too, and the note names it.
func NotePartialPools(checks []Check, poolNote string) {
	if poolNote == "" {
		return
	}
	for i := range checks {
		c := &checks[i]
		if c.Child == domain.GPUNode && c.Parent == domain.GPUPool && len(c.Dangling) > 0 {
			if c.Note != "" {
				c.Note += "; "
			}
			c.Note += poolNote
		}
	}
}
