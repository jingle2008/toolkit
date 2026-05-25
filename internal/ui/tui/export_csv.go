package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
)

// exportRowBuilders dispatches the per-category row-rendering used by
// writeCSV. Mirrors categoryHandlers in table_utils.go but routes
// each cell through ExportRender when set, so OCID-shaped columns
// emit fully-qualified IDs instead of the suffix-only display values.
//
// MUST stay key-aligned with categoryHandlers — any list-view
// category the user can reach with <e> needs an entry here, or the
// export silently emits a header-only CSV. TestExportRowBuilders_
// CoversCategoryHandlers in export_csv_test.go pins that parity.
var exportRowBuilders = map[domain.Category]func(*Model) []table.Row{
	domain.Alias: func(m *Model) []table.Row {
		// Alias is a static enum dump — no dataset access needed and
		// no env-dependent columns to apply ExportRender to.
		return tuiRowsFlatForExport(columns.AliasColumns, domain.Categories,
			"", "", m.curFilter, m.showFaulty)
	},
	domain.Tenant: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.TenantColumns, m.dataset.Tenants,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.LimitDefinition: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.LimitDefinitionColumns, m.dataset.LimitDefinitionGroup.Values,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.ConsolePropertyDefinition: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.ConsolePropertyDefinitionColumns, m.dataset.ConsolePropertyDefinitionGroup.Values,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.PropertyDefinition: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.PropertyDefinitionColumns, m.dataset.PropertyDefinitionGroup.Values,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.LimitTenancyOverride: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.LimitTenancyOverrideColumns, m.dataset.LimitTenancyOverrideMap,
			domain.Tenant, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.ConsolePropertyTenancyOverride: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.ConsolePropertyTenancyOverrideColumns, m.dataset.ConsolePropertyTenancyOverrideMap,
			domain.Tenant, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.PropertyTenancyOverride: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.PropertyTenancyOverrideColumns, m.dataset.PropertyTenancyOverrideMap,
			domain.Tenant, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.LimitRegionalOverride: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.LimitRegionalOverrideColumns, m.dataset.LimitRegionalOverrides,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.ConsolePropertyRegionalOverride: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.ConsolePropertyRegionalOverrideColumns, m.dataset.ConsolePropertyRegionalOverrides,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.PropertyRegionalOverride: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.PropertyRegionalOverrideColumns, m.dataset.PropertyRegionalOverrides,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.BaseModel: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.BaseModelColumns, m.dataset.BaseModels,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.ImportedModel: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.ImportedModelColumns, m.dataset.ImportedModelMap,
			domain.Tenant, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.ModelArtifact: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.ModelArtifactColumns, m.dataset.ModelArtifactMap,
			domain.BaseModel, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.Environment: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.EnvironmentColumns, m.dataset.Environments,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.ServiceTenancy: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.ServiceTenancyColumns, m.dataset.ServiceTenancies,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.GpuPool: func(m *Model) []table.Row {
		return tuiRowsFlatForExport(columns.GpuPoolColumns, m.dataset.GpuPools,
			m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.GpuNode: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.GpuNodeColumns, m.dataset.GpuNodeMap,
			domain.GpuPool, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
	domain.DedicatedAICluster: func(m *Model) []table.Row {
		return tuiRowsGroupedForExport(columns.DacColumns, m.dataset.DedicatedAIClusterMap,
			domain.Tenant, m.context, m.environment.Realm, m.environment.Region, m.curFilter, m.showFaulty)
	},
}

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
// `toolkit get -o csv` exactly. ExportRender on a column produces
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
// (ExportRender preferred over Render), applying the live filter
// and faulty-toggle state. Returns nil if the category has no
// builder registered (e.g. Alias, which the export popup doesn't
// reach today).
func (m *Model) exportRows() []table.Row {
	if builder, ok := exportRowBuilders[m.category]; ok {
		return builder(m)
	}
	return nil
}
