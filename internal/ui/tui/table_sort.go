package tui

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"

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
		common.ContextCol: {},
		// GPU counts are plain integers on all three categories that
		// carry them. Without this they fall to sortByString and order
		// 1, 16, 2, 32, 4, 8. Blank cells parse as 0 and sort with the
		// smallest, matching how the other numeric sorts treat values
		// they can't parse.
		common.GpusCol:     {},
		common.RestartsCol: {},
		common.MinCol:      {},
		common.MaxCol:      {},
	}

	switch {
	case strings.EqualFold(sortColumn, common.AgeCol):
		sortByAge(rows, colIdx, asc)
	case strings.EqualFold(sortColumn, common.UsageCol):
		sortByPercent(rows, colIdx, asc)
	case strings.EqualFold(sortColumn, common.SizeCol):
		sortBySize(rows, colIdx, asc)
	case hasIntHeader(intCols, sortColumn):
		sortByInt(rows, colIdx, asc)
	default:
		sortByString(rows, colIdx, asc)
	}
}

// sortByKey sorts rows by column colIdx under cmp, ascending or
// descending. Rows that tie are ordered by their cells left to right,
// ascending whatever the direction, so ties land in the same order on
// every refresh: grouped categories build their rows from a map, whose
// order changes from one load to the next.
func sortByKey(rows []table.Row, colIdx int, asc bool, cmpCells func(a, b string) int) {
	slices.SortFunc(rows, func(a, b table.Row) int {
		c := cmpCells(a[colIdx], b[colIdx])
		if !asc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return slices.Compare(a, b)
	})
}

func sortByAge(rows []table.Row, colIdx int, asc bool) {
	sortByKey(rows, colIdx, asc, func(a, b string) int {
		return cmp.Compare(k8stime.ParseAge(a), k8stime.ParseAge(b))
	})
}

func sortByInt(rows []table.Row, colIdx int, asc bool) {
	sortByKey(rows, colIdx, asc, func(a, b string) int {
		return cmp.Compare(parseCount(a), parseCount(b))
	})
}

// parseCount parses an integer cell, including the "8×2" per-node ×
// nodes form ServingRuntime's GPUs column uses, which counts as the
// product — the total is what ranks one runtime as bigger than another.
// Unparseable cells are 0.
func parseCount(s string) int64 {
	if perNode, nodes, ok := strings.Cut(s, "×"); ok {
		p, _ := strconv.ParseInt(perNode, 10, 64)
		n, _ := strconv.ParseInt(nodes, 10, 64)
		return p * n
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func sortByPercent(rows []table.Row, colIdx int, asc bool) {
	sortByKey(rows, colIdx, asc, func(a, b string) int {
		av, _ := parsePercent(a)
		bv, _ := parsePercent(b)
		return cmp.Compare(av, bv)
	})
}

// sortBySize sorts rows by size strings like "6B", "3.5M", "1.2T".
func sortBySize(rows []table.Row, colIdx int, asc bool) {
	sortByKey(rows, colIdx, asc, func(a, b string) int {
		av, _ := parseSize(a)
		bv, _ := parseSize(b)
		return cmp.Compare(av, bv)
	})
}

// parseSize parses strings like "6B", "3.5M", "1.2T" into int64.
// 6B/b = 6e9, 3.5M/m = 3.5e6, 1.2T/t = 1.2e12. No suffix = plain number.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n := len(s)
	if n == 0 {
		return 0, nil
	}
	last := s[n-1]
	var mult float64 = 1
	switch last {
	case 'B', 'b':
		mult = 1e9
		s = s[:n-1]
	case 'M', 'm':
		mult = 1e6
		s = s[:n-1]
	case 'T', 't':
		mult = 1e12
		s = s[:n-1]
	}
	num, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	return int64(num*mult + 0.5), nil // round to nearest int
}

func sortByString(rows []table.Row, colIdx int, asc bool) {
	sortByKey(rows, colIdx, asc, strings.Compare)
}

// hasIntHeader checks if the header is in the intHeaders set (case-insensitive).
func hasIntHeader(set map[string]struct{}, header string) bool {
	for k := range set {
		if strings.EqualFold(k, header) {
			return true
		}
	}
	return false
}

// parsePercent parses a string like "37%" as int64 (rounded), returns 0 on error.
//
// Whitespace is trimmed before the "%" is stripped and again after, so
// padded cells (" 37% ", "37 %") parse the same as "37%". Trimming only
// after the suffix strip would leave the "%" attached whenever the value
// had a trailing space, and ParseFloat would reject it.
func parsePercent(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f + 0.5), nil // round to nearest int
}
