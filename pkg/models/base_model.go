// Package models provides data structures for toolkit models.
package models

import (
	"strings"
)

/*
BaseModel represents a base model with its configuration and metadata.
*/
type BaseModel struct {
	Capabilities         []string         `json:"capabilities"`
	InternalName         string           `json:"internalName"`
	Name                 string           `json:"name"`
	DisplayName          string           `json:"displayName"`
	Type                 string           `json:"type"`
	Version              string           `json:"version"`
	Vendor               string           `json:"vendor"`
	MaxTokens            int              `json:"maxTokens"`
	VaultKey             string           `json:"vaultKey"`
	IsExperimental       bool             `json:"isExperimental"`
	IsInternal           bool             `json:"isInternal"`
	LifeCyclePhase       string           `json:"lifeCyclePhase"`
	DeprecatedDate       string           `json:"deprecatedDate,omitempty"`
	OnDemandRetiredDate  string           `json:"onDemandRetiredDate,omitempty"`
	DedicatedRetiredDate string           `json:"dedicatedRetiredDate,omitempty"`
	IsImageTextToText    bool             `json:"isImageTextToText"`
	DACShapeConfigs      *DACShapeConfigs `json:"dacShapeConfigs,omitempty"`
	Runtime              string           `json:"runtime"`
	Replicas             int              `json:"replicas"`
	Status               string           `json:"status"`
	ParameterSize        string           `json:"parameterSize"`
	StorageURI           string           `json:"storageUri,omitempty"`
}

// DACShapeConfigs holds compatible DAC shapes.
type DACShapeConfigs struct {
	CompatibleDACShapes []DACShape `json:"compatibleDACShapes"`
}

// DACShape represents a DAC shape.
type DACShape struct {
	Name      string `json:"name"`
	QuotaUnit int    `json:"quotaUnit"`
	Default   bool   `json:"default"`
}

// GetName returns the name of the base model.
func (m BaseModel) GetName() string {
	return m.Name
}

// GetDefaultDACShape returns the default DAC shape for the base model,
// or nil if none is marked default. If multiple shapes are marked default
// (a malformed config), the first one in declaration order is returned.
//
// The returned pointer aliases an element of the underlying
// CompatibleDACShapes slice, which is reached through the *DACShapeConfigs
// pointer field. Mutating the pointed-to DACShape will be visible to every
// BaseModel value that shares the same DACShapeConfigs. Treat the result
// as read-only.
func (m BaseModel) GetDefaultDACShape() *DACShape {
	if m.DACShapeConfigs == nil {
		return nil
	}
	for i := range m.DACShapeConfigs.CompatibleDACShapes {
		shape := &m.DACShapeConfigs.CompatibleDACShapes[i]
		if shape.Default {
			return shape
		}
	}
	return nil
}

// GetFilterableFields returns filterable fields for the base model.
func (m BaseModel) GetFilterableFields() []string {
	var shapeName string
	shape := m.GetDefaultDACShape()
	if shape != nil {
		shapeName = shape.Name
	}

	return append(m.Capabilities, m.Name, m.DisplayName, m.Status,
		m.Type, m.GetFlags(), shapeName, m.Runtime, m.StorageURI)
}

// IsFaulty reports whether the model's Status is anything other than "Ready".
func (m BaseModel) IsFaulty() bool {
	return m.Status != "Ready"
}

// GetFlags returns the flags for the base model.
func (m BaseModel) GetFlags() string {
	flags := []string{}
	if m.IsExperimental {
		flags = append(flags, "EXP")
	}
	if m.IsInternal {
		flags = append(flags, "INT")
	}
	switch m.LifeCyclePhase {
	case "DEPRECATED":
		flags = append(flags, "RTD")
	case "ONDEMAND_SERVING_DISABLED":
		flags = append(flags, "DAC")
	}

	if m.IsImageTextToText {
		flags = append(flags, "IMG")
	}

	return strings.Join(flags, "/")
}
