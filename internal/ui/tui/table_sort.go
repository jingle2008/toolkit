package tui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	k8stime "github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// sortRows sorts rows in-place by column & direction.
func sortRows(rows []table.Row, headers []header, sortColumn string, asc bool) {
	colIdx := slices.IndexFunc(headers, func(h header) bool {
		return strings.EqualFold(h.text, sortColumn)
	})
	if colIdx < 0 {
		return
	}

	intCols := map[string]struct{}{
		common.FreeCol:    {},
		common.SizeCol:    {},
		common.ContextCol: {},
	}

	switch {
	case strings.EqualFold(sortColumn, common.AgeCol):
		sortByAge(rows, colIdx, asc)
	case strings.EqualFold(sortColumn, common.UsageCol):
		sortByPercent(rows, colIdx, asc)
	case hasIntHeader(intCols, sortColumn):
		sortByInt(rows, colIdx, asc)
	default:
		sortByString(rows, colIdx, asc)
	}
}

func sortByAge(rows []table.Row, colIdx int, asc bool) {
	slices.SortFunc(rows, func(a, b table.Row) int {
		av := k8stime.ParseAge(a[colIdx])
		bv := k8stime.ParseAge(b[colIdx])
		if asc {
			return int(av - bv)
		}
		return int(bv - av)
	})
}

func sortByInt(rows []table.Row, colIdx int, asc bool) {
	slices.SortFunc(rows, func(a, b table.Row) int {
		av, _ := strconv.ParseInt(a[colIdx], 10, 64)
		bv, _ := strconv.ParseInt(b[colIdx], 10, 64)
		if asc {
			return int(av - bv)
		}
		return int(bv - av)
	})
}

func sortByPercent(rows []table.Row, colIdx int, asc bool) {
	slices.SortFunc(rows, func(a, b table.Row) int {
		av, _ := parsePercent(a[colIdx])
		bv, _ := parsePercent(b[colIdx])
		if asc {
			return int(av - bv)
		}
		return int(bv - av)
	})
}

func sortByString(rows []table.Row, colIdx int, asc bool) {
	slices.SortFunc(rows, func(a, b table.Row) int {
		if asc {
			return strings.Compare(a[colIdx], b[colIdx])
		}
		return strings.Compare(b[colIdx], a[colIdx])
	})
}

// hasIntHeader checks if the header is in the intHeaders map (case-insensitive).
func hasIntHeader(m map[string]struct{}, header string) bool {
	for k := range m {
		if strings.EqualFold(k, header) {
			return true
		}
	}
	return false
}

// parsePercent parses a string like "37%" as int64 (rounded), returns 0 on error.
func parsePercent(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f + 0.5), nil // round to nearest int
}
