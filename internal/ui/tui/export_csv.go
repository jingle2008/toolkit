package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/cli/output"
)

/*
exportTableCSV writes the current table data (with headers) to the given file path.
Returns nil on success, or an error.
*/
func (m *Model) exportTableCSV(outPath string) (err error) {
	if m == nil || m.table == nil {
		return fmt.Errorf("no table data to export")
	}
	outPath = filepath.Clean(outPath)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() {
		cerr := f.Close()
		if err == nil && cerr != nil {
			err = cerr
		}
	}()
	err = m.writeCSV(f)
	return
}

// writeCSV emits the current filtered table data (with headers) to w.
// Routes through output.WriteDelimited so the on-wire format matches
// `toolkit get -o csv` exactly. RenderForExport on a column produces
// fully-qualified OCIDs in place of raw suffixes — see the
// applyMiddleTruncation neighbor for the corresponding display-mode
// behaviour. Sort order is the dataset's natural order (the live
// table's interactive sort is not preserved on export — the filter
// and the faulty-toggle are).
func (m *Model) writeCSV(w io.Writer) error {
	headers := make([]string, len(m.headers))
	for i, h := range m.headers {
		headers[i] = h.text
	}
	rows := m.exportRows()
	strRows := make([][]string, len(rows))
	for i, r := range rows {
		strRows[i] = []string(r)
	}
	return output.WriteDelimited(w, headers, strRows, output.Options{}, ',')
}

// exportRows rebuilds rows for the current category in export mode
// (RenderForExport preferred over Render), applying the live filter
// and faulty-toggle state. Returns nil if no rowSource is
// registered for the category. Shares its dispatch with the live
// table via rowSources — see row_sources.go.
func (m *Model) exportRows() []table.Row {
	src, ok := rowSources[m.category]
	if !ok {
		return nil
	}
	return src.rows(rowCtx{
		dataset: m.dataset,
		scope:   m.scope,
		realm:   m.environment.Realm,
		region:  m.environment.Region,
		filter:  m.curFilter,
		faulty:  m.showFaulty,
		export:  true,
	})
}
