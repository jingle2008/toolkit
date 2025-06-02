package toolkit

type header struct {
	text  string
	ratio float64
}

var headerDefinitions = map[Category][]header{
	Tenant: {
		{text: "Name", ratio: 0.25},
		{text: "OCID", ratio: 0.65},
		{text: "LO/CPO/PO", ratio: 0.1},
	},
	LimitDefinition: {
		{text: "Name", ratio: 0.32},
		{text: "Description", ratio: 0.48},
		{text: "Scope", ratio: 0.08},
		{text: "Min", ratio: 0.06},
		{text: "Max", ratio: 0.06},
	},
	ConsolePropertyDefinition: {
		{text: "Name", ratio: 0.38},
		{text: "Description", ratio: 0.5},
		{text: "Value", ratio: 0.12},
	},
	PropertyDefinition: {
		{text: "Name", ratio: 0.38},
		{text: "Description", ratio: 0.5},
		{text: "Value", ratio: 0.12},
	},
	LimitTenancyOverride: {
		{text: "Tenant", ratio: 0.24},
		{text: "Limit", ratio: 0.4},
		{text: "Regions", ratio: 0.2},
		{text: "Min", ratio: 0.08},
		{text: "Max", ratio: 0.08},
	},
	ConsolePropertyTenancyOverride: {
		{text: "Tenant", ratio: 0.25},
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.25},
		{text: "Value", ratio: 0.1},
	},
	PropertyTenancyOverride: {
		{text: "Tenant", ratio: 0.25},
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.25},
		{text: "Value", ratio: 0.1},
	},
	ConsolePropertyRegionalOverride: {
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.3},
		{text: "Value", ratio: 0.3},
	},
	PropertyRegionalOverride: {
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.3},
		{text: "Value", ratio: 0.3},
	},
	BaseModel: {
		{text: "Name", ratio: 0.26},
		{text: "Version", ratio: 0.08},
		{text: "Type", ratio: 0.08},
		{text: "DAC Shape", ratio: 0.16},
		{text: "Capabilities", ratio: 0.18},
		{text: "Category", ratio: 0.08},
		{text: "Max Tokens", ratio: 0.08},
		{text: "Flags", ratio: 0.08},
	},
	ModelArtifact: {
		{text: "Model Name", ratio: 0.3},
		{text: "GPU Config", ratio: 0.1},
		{text: "Artifact Name", ratio: 0.5},
		{text: "TRT Version", ratio: 0.1},
	},
	Environment: {
		{text: "Name", ratio: 0.2},
		{text: "Realm", ratio: 0.15},
		{text: "Type", ratio: 0.15},
		{text: "Region", ratio: 0.5},
	},
	ServiceTenancy: {
		{text: "Name", ratio: 0.15},
		{text: "Realm", ratio: 0.1},
		{text: "Environment", ratio: 0.1},
		{text: "Home Region", ratio: 0.15},
		{text: "Regions", ratio: 0.5},
	},
	GpuPool: {
		{text: "Name", ratio: 0.3},
		{text: "Shape", ratio: 0.3},
		{text: "Size", ratio: 0.1},
		{text: "GPUs", ratio: 0.1},
		{text: "OKE Managed", ratio: 0.1},
		{text: "Capacity Type", ratio: 0.1},
	},
	GpuNode: {
		{text: "PoolName", ratio: 0.2},
		{text: "Name", ratio: 0.15},
		{text: "Instance Type", ratio: 0.15},
		{text: "Total", ratio: 0.08},
		{text: "Free", ratio: 0.08},
		{text: "Healthy", ratio: 0.08},
		{text: "Ready", ratio: 0.08},
		{text: "Status", ratio: 0.18},
	},
	DedicatedAICluster: {
		{text: "Tenant", ratio: 0.2},
		{text: "Name", ratio: 0.44},
		{text: "Type", ratio: 0.07},
		{text: "Unit Shape/Profile", ratio: 0.16},
		{text: "Size", ratio: 0.05},
		{text: "Status", ratio: 0.08},
	},
}
