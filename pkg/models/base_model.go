package models

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

type BaseModel struct {
	Capabilities        map[string]*Capability `json:"capabilities"`
	InternalName        string                 `json:"internal_name"`
	Name                string                 `json:"displayName"`
	Type                string                 `json:"type"`
	Category            string                 `json:"category"`
	Version             string                 `json:"version"`
	Vendor              string                 `json:"vendor"`
	MaxTokens           int                    `json:"maxTokens"`
	VaultKey            string                 `json:"vaultKey"`
	IsExperimental      bool                   `json:"isExperimental"`
	IsInternal          bool                   `json:"isInternal"`
	IsLongTermSupported bool                   `json:"isLongTermSupported"`
	LifeCyclePhase      string                 `json:"baseModelLifeCyclePhase"`
	TimeDeprecated      string                 `json:"timeDeprecated"`
}

type Capability struct {
	Capability  string       `json:"capability"`
	CrName      string       `json:"cr_name"`
	Description string       `json:"description"`
	Runtime     string       `json:"runtime"`
	ValuesFile  *string      `json:"values_file,omitempty"`
	ChartValues *ChartValues `json:"chart_values,omitempty"`
	Replicas    int          `json:"replicas"`
}

type ChartValues struct {
	Model         *ModelSetting  `json:"model,omitempty"`
	ModelMetaData *ModelMetaData `json:"modelMetaData,omitempty"`
}

type ModelSetting struct {
	ModelMaxLoadingSeconds *string `yaml:"modelMaxLoadingSeconds" json:"modelMaxLoadingSeconds,omitempty"`
}

type ModelMetaData struct {
	DacShapeConfigs         *DacShapeConfigs         `json:"dacShapeConfigs,omitempty"`
	TrainingConfigs         *TrainingConfigs         `json:"trainingConfigs,omitempty"`
	ServingBaseModelConfigs *ServingBaseModelConfigs `json:"servingBaseModelConfigs,omitempty"`
}

type DacShapeConfigs struct {
	CompatibleDACShapes []DACShape `yaml:"compatibleDACShapes" json:"compatibleDACShapes"`
}

type DACShape struct {
	Name      string `yaml:"name" json:"name"`
	QuotaUnit int    `yaml:"quotaUnit" json:"quotaUnit"`
	Default   bool   `yaml:"default" json:"default"`
}

type TrainingConfigs struct {
	CompatibleTrainingConfigs []TrainingConfig `yaml:"compatibleTrainingConfigs" json:"compatibleTrainingConfigs"`
}

type TrainingConfig struct {
	Name                string `yaml:"name" json:"name"`
	SupportStackServing bool   `yaml:"supportStackServing" json:"supportStackServing"`
	Default             bool   `yaml:"default" json:"default"`
}

type ServingBaseModelConfigs struct {
	ServingBaseModel ServingBaseModel `yaml:"servingBaseModel" json:"servingBaseModel"`
}

type ServingBaseModel struct {
	CRName       string `yaml:"cr_name" json:"cr_name"`
	InternalName string `yaml:"internal_name" json:"internal_name"`
}

func (m BaseModel) GetName() string {
	return m.Name
}

func (m BaseModel) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", m.Type, m.Name, m.Version)
}

func (m BaseModel) GetCapabilities() []string {
	keys := make([]string, 0, len(m.Capabilities))
	for key, value := range m.Capabilities {
		cap := key
		if value.Replicas > 0 {
			cap = fmt.Sprintf("[%d] %s", value.Replicas, key)
		}
		keys = append(keys, cap)
	}

	sort.Strings(keys)
	return keys
}

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
		log.Fatalf("More than 1 shapes found for model: %s", m.GetKey())
	}

	for _, value := range shapes {
		return value
	}

	return nil
}

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
