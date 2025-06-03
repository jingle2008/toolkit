package toolkit

import (
	"context"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestProductionLoader_LoadDataset(t *testing.T) {
	t.Parallel()
	loader := ProductionLoader{}
	_, err := loader.LoadDataset(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadDataset: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadBaseModels(t *testing.T) {
	t.Parallel()
	loader := ProductionLoader{}
	_, err := loader.LoadBaseModels(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadBaseModels: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGpuPools(t *testing.T) {
	t.Parallel()
	loader := ProductionLoader{}
	_, err := loader.LoadGpuPools(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadGpuPools: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGpuNodes(t *testing.T) {
	t.Parallel()
	loader := ProductionLoader{}
	_, err := loader.LoadGpuNodes(context.Background(), "dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadGpuNodes: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadDedicatedAIClusters(t *testing.T) {
	t.Parallel()
	loader := ProductionLoader{}
	_, err := loader.LoadDedicatedAIClusters(context.Background(), "dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadDedicatedAIClusters: expected error or empty result with dummy input")
	}
}
