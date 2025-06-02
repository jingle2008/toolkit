package toolkit

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestProductionLoader_LoadDataset(t *testing.T) {
	loader := ProductionLoader{}
	_, err := loader.LoadDataset("dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadDataset: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadBaseModels(t *testing.T) {
	loader := ProductionLoader{}
	_, err := loader.LoadBaseModels("dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadBaseModels: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGpuPools(t *testing.T) {
	loader := ProductionLoader{}
	_, err := loader.LoadGpuPools("dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadGpuPools: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGpuNodes(t *testing.T) {
	loader := ProductionLoader{}
	_, err := loader.LoadGpuNodes("dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadGpuNodes: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadDedicatedAIClusters(t *testing.T) {
	loader := ProductionLoader{}
	_, err := loader.LoadDedicatedAIClusters("dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadDedicatedAIClusters: expected error or empty result with dummy input")
	}
}
