package tui

import (
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

type header struct {
	text  string
	ratio float64
}

var headerDefinitions = map[domain.Category][]header{
	domain.Alias: {
		{common.NameCol, 0.4},
		{"Aliases", 0.6},
	},
	domain.Tenant: {
		{common.NameCol, 0.20},
		{"OCID", 0.60},
		{"Internal", 0.1},
		{"Note", 0.1},
	},
	domain.LimitDefinition: {
		{common.NameCol, 0.32},
		{"Description", 0.48},
		{"Scope", 0.08},
		{"Min", 0.06},
		{"Max", 0.06},
	},
	domain.ConsolePropertyDefinition: {
		{common.NameCol, 0.38},
		{"Description", 0.5},
		{common.ValueCol, 0.12},
	},
	domain.PropertyDefinition: {
		{common.NameCol, 0.38},
		{"Description", 0.5},
		{common.ValueCol, 0.12},
	},
	domain.LimitTenancyOverride: {
		{common.NameCol, 0.4},
		{common.TenantCol, 0.24},
		{common.RegionsCol, 0.2},
		{"Min", 0.08},
		{"Max", 0.08},
	},
	domain.ConsolePropertyTenancyOverride: {
		{common.NameCol, 0.4},
		{common.TenantCol, 0.25},
		{common.RegionsCol, 0.25},
		{common.ValueCol, 0.1},
	},
	domain.PropertyTenancyOverride: {
		{common.NameCol, 0.4},
		{common.TenantCol, 0.25},
		{common.RegionsCol, 0.25},
		{common.ValueCol, 0.1},
	},
	domain.LimitRegionalOverride: {
		{common.NameCol, 0.4},
		{common.RegionsCol, 0.3},
		{"Min", 0.15},
		{"Max", 0.15},
	},
	domain.ConsolePropertyRegionalOverride: {
		{common.NameCol, 0.4},
		{common.RegionsCol, 0.4},
		{common.ValueCol, 0.2},
	},
	domain.PropertyRegionalOverride: {
		{common.NameCol, 0.4},
		{common.RegionsCol, 0.4},
		{common.ValueCol, 0.2},
	},
	domain.BaseModel: {
		{common.NameCol, 0.26},
		{"Display Name", 0.28},
		{"Version", 0.06},
		{"DAC Shape", 0.15},
		{"Size", 0.06},
		{common.ContextCol, 0.08},
		{"Flags", 0.07},
		{"Status", 0.04},
	},
	domain.ModelArtifact: {
		{common.NameCol, 0.5},
		{"Model Internal Name", 0.3},
		{"GPU Config", 0.1},
		{"TensorRT", 0.1},
	},
	domain.Environment: {
		{common.NameCol, 0.2},
		{"Realm", 0.15},
		{common.TypeCol, 0.15},
		{"Region", 0.5},
	},
	domain.ServiceTenancy: {
		{common.NameCol, 0.15},
		{"Realm", 0.1},
		{common.TypeCol, 0.1},
		{"Home Region", 0.15},
		{common.RegionsCol, 0.5},
	},
	domain.GpuPool: {
		{common.NameCol, 0.3},
		{"Shape", 0.3},
		{common.SizeCol, 0.1},
		{"GPUs", 0.1},
		{"OKE Managed", 0.1},
		{"Capacity Type", 0.1},
	},
	domain.GpuNode: {
		{common.NameCol, 0.15},
		{"Pool", 0.22},
		{common.TypeCol, 0.15},
		{"Total", 0.06},
		{common.FreeCol, 0.06},
		{"Healthy", 0.06},
		{"Ready", 0.06},
		{common.AgeCol, 0.06},
		{"Status", 0.18},
	},
	domain.DedicatedAICluster: {
		{common.NameCol, 0.42},
		{common.TenantCol, 0.16},
		{"Internal", 0.05},
		{common.UsageCol, 0.05},
		{common.TypeCol, 0.06},
		{"Shape/Profile", 0.13},
		{common.SizeCol, 0.04},
		{common.AgeCol, 0.04},
		{"Status", 0.05},
	},
}
