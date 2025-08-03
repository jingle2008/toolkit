// Package models provides data structures for toolkit models.
package models

import (
	"fmt"
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
	DacShapeConfigs      *DacShapeConfigs `json:"dacShapeConfigs,omitempty"`
	Runtime              string           `json:"runtime"`
	Replicas             int              `json:"replicas"`
	Status               string           `json:"status"`
	ParameterSize        string           `json:"parameterSize"`
}

// DacShapeConfigs holds compatible DAC shapes.
type DacShapeConfigs struct {
	CompatibleDACShapes []DACShape `yaml:"compatibleDACShapes" json:"compatibleDACShapes"`
}

// DACShape represents a DAC shape.
type DACShape struct {
	Name      string `yaml:"name" json:"name"`
	QuotaUnit int    `yaml:"quotaUnit" json:"quotaUnit"`
	Default   bool   `yaml:"default" json:"default"`
}

// GetName returns the name of the base model.
func (m BaseModel) GetName() string {
	return m.Name
}

// GetDefaultDacShape returns the default DAC shape for the base model.
func (m BaseModel) GetDefaultDacShape() *DACShape {
	shapes := make(map[string]*DACShape)

	if m.DacShapeConfigs != nil {
		for _, config := range m.DacShapeConfigs.CompatibleDACShapes {
			if config.Default {
				shapes[config.Name] = &config
			}
		}
	}

	if len(shapes) > 1 {
		panic(fmt.Sprintf("More than 1 default DAC shapes found for model: %s", m.InternalName))
	}

	for _, value := range shapes {
		return value
	}

	return nil
}

// GetFilterableFields returns filterable fields for the base model.
func (m BaseModel) GetFilterableFields() []string {
	var shapeName string
	shape := m.GetDefaultDacShape()
	if shape != nil {
		shapeName = shape.Name
	}

	return append(m.Capabilities, m.Name, m.DisplayName, m.Status,
		m.Type, m.GetFlags(), shapeName, m.Runtime)
}

// IsFaulty returns false by default for BaseModel.
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
