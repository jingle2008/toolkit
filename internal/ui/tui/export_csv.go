package tui

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
exportTableCSV writes the current table data (with headers) to the given file path.
Returns nil on success, or an error.
*/
//nolint:cyclop // function is clear and further splitting would reduce readability
func (m *Model) exportTableCSV(outPath string) error {
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

	w := csv.NewWriter(f)
	defer w.Flush()

	// Write headers
	headers := make([]string, 0, len(m.headers))
	for _, h := range m.headers {
		headers = append(headers, h.text)
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	// Write rows
	rows := m.table.Rows()
	if m.category == domain.DedicatedAICluster {
		realm := m.environment.Realm
		region := m.environment.Region
		rows = filterRowsScoped(
			m.dataset.DedicatedAIClusterMap, domain.Tenant,
			m.context, m.curFilter, m.showFaulty,
			func(val models.DedicatedAICluster, _ string) table.Row {
				id := val.GetID(realm, region)
				return dedicatedAIClusterToRowInternal(
					val, val.GetTenantID(realm), &id)
			})
	}

	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}
