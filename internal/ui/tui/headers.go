package tui

import "github.com/jingle2008/toolkit/internal/domain"

type header struct {
	text  string
	ratio float64
}

var headerDefinitions = map[domain.Category][]header{
	domain.Tenant: {
		{"Name", 0.20},
		{"OCID", 0.60},
		{"Internal", 0.1},
		{"Note", 0.1},
	},
	domain.LimitDefinition: {
		{"Name", 0.32},
		{"Description", 0.48},
		{"Scope", 0.08},
		{"Min", 0.06},
		{"Max", 0.06},
	},
	domain.ConsolePropertyDefinition: {
		{"Name", 0.38},
		{"Description", 0.5},
		{"Value", 0.12},
	},
	domain.PropertyDefinition: {
		{"Name", 0.38},
		{"Description", 0.5},
		{"Value", 0.12},
	},
	domain.LimitTenancyOverride: {
		{"Tenant", 0.24},
		{"Name", 0.4},
		{"Regions", 0.2},
		{"Min", 0.08},
		{"Max", 0.08},
	},
	domain.ConsolePropertyTenancyOverride: {
		{"Tenant", 0.25},
		{"Name", 0.4},
		{"Regions", 0.25},
		{"Value", 0.1},
	},
	domain.PropertyTenancyOverride: {
		{"Tenant", 0.25},
		{"Name", 0.4},
		{"Regions", 0.25},
		{"Value", 0.1},
	},
	domain.LimitRegionalOverride: {
		{"Name", 0.4},
		{"Regions", 0.3},
		{"Min", 0.15},
		{"Max", 0.15},
	},
	domain.ConsolePropertyRegionalOverride: {
		{"Name", 0.4},
		{"Regions", 0.4},
		{"Value", 0.2},
	},
	domain.PropertyRegionalOverride: {
		{"Name", 0.4},
		{"Regions", 0.4},
		{"Value", 0.2},
	},
	domain.BaseModel: {
		{"Internal Name", 0.28},
		{"Name", 0.26},
		{"Version", 0.08},
		{"DAC Shape", 0.16},
		{"Caps", 0.06},
		{"Max Tokens", 0.08},
		{"Flags", 0.08},
	},
	domain.ModelArtifact: {
		{"Model", 0.3},
		{"GPU Config", 0.1},
		{"Name", 0.5},
		{"TensorRT", 0.1},
	},
	domain.Environment: {
		{"Name", 0.2},
		{"Realm", 0.15},
		{"Type", 0.15},
		{"Region", 0.5},
	},
	domain.ServiceTenancy: {
		{"Name", 0.15},
		{"Realm", 0.1},
		{"Environment", 0.1},
		{"Home Region", 0.15},
		{"Regions", 0.5},
	},
	domain.GpuPool: {
		{"Name", 0.3},
		{"Shape", 0.3},
		{"Size", 0.1},
		{"GPUs", 0.1},
		{"OKE Managed", 0.1},
		{"Capacity Type", 0.1},
	},
	domain.GpuNode: {
		{"Pool", 0.22},
		{"Name", 0.15},
		{"Type", 0.15},
		{"Total", 0.06},
		{"Free", 0.06},
		{"Healthy", 0.06},
		{"Ready", 0.06},
		{"Age", 0.06},
		{"Status", 0.18},
	},
	domain.DedicatedAICluster: {
		{"Tenant", 0.18},
		{"Name", 0.42},
		{"Internal", 0.05},
		{"Usage", 0.05},
		{"Type", 0.06},
		{"Shape/Profile", 0.15},
		{"Size", 0.04},
		{"Status", 0.05},
	},
}
