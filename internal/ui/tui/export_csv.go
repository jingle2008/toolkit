package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/table"

	"github.com/jingle2008/toolkit/internal/output"
)

// csvExport is a table's CSV content — upper-cased headers and the
// export-mode rows — built on the Update loop, so the file write that
// runs as a tea.Cmd reads nothing of the model.
type csvExport struct {
	headers []string
	rows    [][]string
}

/*
exportTableCSV writes the current table data (with headers) to the given file path.
Returns nil on success, or an error.
*/
func (m *Model) exportTableCSV(outPath string) error {
	data, err := m.csvSnapshot()
	if err != nil {
		return err
	}
	return data.writeFile(outPath)
}

// csvSnapshot captures what exportTableCSV writes, or why there is
// nothing to write.
func (m *Model) csvSnapshot() (csvExport, error) {
	if m == nil || m.table == nil {
		return csvExport{}, fmt.Errorf("no table data to export")
	}
	return m.csvData(), nil
}

// writeFile writes the CSV to outPath.
func (c csvExport) writeFile(outPath string) (err error) {
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
	err = c.write(f)
	return
}

// write emits the CSV to w. Routes through output.WriteDelimited so the
// on-wire format matches `toolkit get -o csv` exactly.
func (c csvExport) write(w io.Writer) error {
	return output.WriteDelimited(w, c.headers, c.rows, output.Options{}, ',')
}

// writeCSV emits the current filtered table data (with headers) to w.
func (m *Model) writeCSV(w io.Writer) error {
	return m.csvData().write(w)
}

// csvData builds the current filtered table data (with headers).
// RenderForExport on a column produces fully-qualified OCIDs in place
// of raw suffixes — see the
// applyMiddleTruncation neighbor for the corresponding display-mode
// behaviour. Sort order is the dataset's natural order (the live
// table's interactive sort is not preserved on export — the filter
// and the faulty-toggle are). Headers are upper-cased as the CLI's are
// (columns.RenderTable), so the bytes match.
func (m *Model) csvData() csvExport {
	headers := make([]string, len(m.headers))
	for i, h := range m.headers {
		headers[i] = strings.ToUpper(h.text)
	}
	rows := m.exportRows()
	strRows := make([][]string, len(rows))
	for i, r := range rows {
		strRows[i] = []string(r)
	}
	return csvExport{headers: headers, rows: strRows}
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
		dataset:  m.dataset,
		category: m.category,
		scope:    m.scope,
		realm:    m.environment.Realm,
		region:   m.environment.Region,
		filter:   m.filter,
		faulty:   m.showFaulty,
		export:   true,
	})
}
