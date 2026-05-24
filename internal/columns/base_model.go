package columns

import (
	"fmt"
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// baseModelDacShape formats the default DAC shape as "{QuotaUnit}x {Name}",
// or returns "" when no default shape is configured.
func baseModelDacShape(m models.BaseModel) string {
	shape := m.GetDefaultDacShape()
	if shape == nil {
		return ""
	}
	return fmt.Sprintf("%dx %s", shape.QuotaUnit, shape.Name)
}

// BaseModelColumns is the canonical column set for domain.BaseModel.
// Default==true columns match today's CLI (Name, Internal, Vendor, Type,
// Version, Flags, Status). Display Name, DAC Shape, Size, Context are
// Default==false (TUI-only opt-in via --columns).
//
// Ratios sum to 1.00. They diverge from today's TUI headerDefinitions
// because the canonical set unions the CLI's columns (Internal, Vendor,
// Type) with the TUI's (Display Name, DAC Shape, Size, Context); the
// TUI's 8-column 1.00 budget was rebalanced across the 11-column union.
var BaseModelColumns = Set[models.BaseModel]{Columns: []Column[models.BaseModel]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.18,
		Render: func(m models.BaseModel) string { return m.Name }},
	{Title: "Display Name", Key: "display-name", Default: false, Ratio: 0.20,
		Render: func(m models.BaseModel) string { return m.DisplayName }},
	{Title: "Internal", Key: "internal", Default: true, Ratio: 0.12,
		Render: func(m models.BaseModel) string { return m.InternalName }},
	{Title: "Vendor", Key: "vendor", Default: true, Ratio: 0.07,
		Render: func(m models.BaseModel) string { return m.Vendor }},
	{Title: "Type", Key: "type", Default: true, Ratio: 0.05,
		Render: func(m models.BaseModel) string { return m.Type }},
	{Title: "Version", Key: "version", Default: true, Ratio: 0.06,
		Render: func(m models.BaseModel) string { return m.Version }},
	{Title: "DAC Shape", Key: "dac-shape", Default: false, Ratio: 0.10,
		Render: func(m models.BaseModel) string { return baseModelDacShape(m) }},
	{Title: "Size", Key: "size", Default: false, Ratio: 0.05,
		Render: func(m models.BaseModel) string { return m.ParameterSize }},
	{Title: "Context", Key: "context", Default: false, Ratio: 0.05,
		Render: func(m models.BaseModel) string { return strconv.Itoa(m.MaxTokens) }},
	{Title: "Flags", Key: "flags", Default: true, Ratio: 0.07,
		Render: func(m models.BaseModel) string { return m.GetFlags() }},
	{Title: "Status", Key: "status", Default: true, Ratio: 0.05,
		Render: func(m models.BaseModel) string { return m.Status }},
}}
