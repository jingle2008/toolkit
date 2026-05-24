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
// 8 columns, ratios sum to 1.00. Internal/Vendor/Type were dropped in
// favor of Display Name + DAC Shape + Size + Context, which carry more
// operator-useful information; consumers that still need the dropped
// fields can read them from the JSON struct (`-o json`).
var BaseModelColumns = Set[models.BaseModel]{Columns: []Column[models.BaseModel]{
	{Title: "Name", Key: "name", Ratio: 0.22,
		Render: func(m models.BaseModel) string { return m.Name }},
	{Title: "Display Name", Key: "display-name", Ratio: 0.26,
		Render: func(m models.BaseModel) string { return m.DisplayName }},
	{Title: "Version", Key: "version", Ratio: 0.08,
		Render: func(m models.BaseModel) string { return m.Version }},
	{Title: "DAC Shape", Key: "dac-shape", Ratio: 0.14,
		Render: func(m models.BaseModel) string { return baseModelDacShape(m) }},
	{Title: "Size", Key: "size", Ratio: 0.07,
		Render: func(m models.BaseModel) string { return m.ParameterSize }},
	{Title: "Context", Key: "context", Ratio: 0.07,
		Render: func(m models.BaseModel) string { return strconv.Itoa(m.MaxTokens) }},
	{Title: "Flags", Key: "flags", Ratio: 0.09,
		Render: func(m models.BaseModel) string { return m.GetFlags() }},
	{Title: "Status", Key: "status", Ratio: 0.07,
		Render: func(m models.BaseModel) string { return m.Status }},
}}
