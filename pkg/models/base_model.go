// Package models provides data structures for toolkit models.
package models

import (
	"fmt"
	"sort"
	"strings"
)

// BaseModel represents a base model entity.
type BaseModel struct {
	Capabilities           map[string]*Capability `json:"capabilities"`
	InternalName           string                 `json:"internal_name"`
	Name                   string                 `json:"displayName"`
	Type                   string                 `json:"type"`
	Category               string                 `json:"category"`
	Version                string                 `json:"version"`
	Vendor                 string                 `json:"vendor"`
	MaxTokens              int                    `json:"maxTokens"`
	VaultKey               string                 `json:"vaultKey"`
	IsExperimental         bool                   `json:"isExperimental"`
	IsInternal             bool                   `json:"isInternal"`
	IsLongTermSupported    bool                   `json:"isLongTermSupported"`
	LifeCyclePhase         string                 `json:"baseModelLifeCyclePhase"`
	TimeDeprecated         string                 `json:"timeDeprecated"`
	ImageTextToText        *bool                  `json:"imageTextToText,omitempty"`
	ContainerImageOverride *string                `json:"containerImageOverride,omitempty"`
}

// Capability represents a model capability.
type Capability struct {
	Capability        string       `json:"capability"`
	CrName            string       `json:"cr_name"`
	Description       string       `json:"description"`
	Runtime           string       `json:"runtime"`
	ValuesFile        *string      `json:"values_file,omitempty"`
	ChartValues       *ChartValues `json:"chart_values,omitempty"`
	Replicas          int          `json:"replicas"`
	MaxLoadingSeconds *string      `json:"max_loading_seconds,omitempty"`
}

// ChartValues holds chart configuration values for a model.
type ChartValues struct {
	Model         *ModelSetting  `json:"model,omitempty"`
	ModelMetaData *ModelMetaData `json:"modelMetaData,omitempty"`
}

// ModelSetting holds settings for a model.
type ModelSetting struct {
	ModelMaxLoadingSeconds *string `yaml:"modelMaxLoadingSeconds" json:"modelMaxLoadingSeconds,omitempty"`
}

// ModelMetaData holds metadata for a model.
type ModelMetaData struct {
	DacShapeConfigs         *DacShapeConfigs         `json:"dacShapeConfigs,omitempty"`
	TrainingConfigs         *TrainingConfigs         `json:"trainingConfigs,omitempty"`
	ServingBaseModelConfigs *ServingBaseModelConfigs `json:"servingBaseModelConfigs,omitempty"`
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

// TrainingConfigs holds compatible training configurations.
type TrainingConfigs struct {
	CompatibleTrainingConfigs []TrainingConfig `yaml:"compatibleTrainingConfigs" json:"compatibleTrainingConfigs"`
}

// TrainingConfig represents a training configuration.
type TrainingConfig struct {
	Name                string `yaml:"name" json:"name"`
	SupportStackServing bool   `yaml:"supportStackServing" json:"supportStackServing"`
	Default             bool   `yaml:"default" json:"default"`
}

// ServingBaseModelConfigs holds serving base model configurations.
type ServingBaseModelConfigs struct {
	ServingBaseModel ServingBaseModel `yaml:"servingBaseModel" json:"servingBaseModel"`
}

// ServingBaseModel represents a serving base model.
type ServingBaseModel struct {
	CRName       string `yaml:"cr_name" json:"cr_name"`
	InternalName string `yaml:"internal_name" json:"internal_name"`
}

// GetName returns the name of the base model.
func (m BaseModel) GetName() string {
	return m.Name
}

// GetKey returns the key of the base model.
func (m BaseModel) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", m.Type, m.Name, m.Version)
}

// GetCapabilities returns the capabilities of the base model.
func (m BaseModel) GetCapabilities() []string {
	keys := make([]string, 0, len(m.Capabilities))
	for key, value := range m.Capabilities {
		capStr := strings.ToUpper(key[:1])
		if value.Replicas > 0 {
			capStr = fmt.Sprintf("%s*%d", capStr, value.Replicas)
		}
		keys = append(keys, capStr)
	}

	sort.Strings(keys)
	return keys
}

// GetDefaultDacShape returns the default DAC shape for the base model.
func (m BaseModel) GetDefaultDacShape() *DACShape {
	shapes := make(map[string]*DACShape)
	for _, value := range m.Capabilities {
		if value.ChartValues == nil {
			continue
		}

		for _, config := range value.ChartValues.ModelMetaData.DacShapeConfigs.CompatibleDACShapes {
			if config.Default {
				shapes[config.Name] = &config
			}
		}
	}

	if len(shapes) > 1 {
		panic(fmt.Sprintf("More than 1 default DAC shapes found for model: %s", m.GetKey()))
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

	return append(m.GetCapabilities(), m.Name,
		m.Type, m.Category, m.Vendor, m.Version,
		m.GetFlags(), shapeName)
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
	if m.IsLongTermSupported {
		flags = append(flags, "LTS")
	}
	if m.LifeCyclePhase == "DEPRECATED" {
		flags = append(flags, "RTD")
	}
	if m.LifeCyclePhase == "ONDEMAND_SERVING_DISABLED" {
		flags = append(flags, "DAC")
	}

	return strings.Join(flags, "/")
}
