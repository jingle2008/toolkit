package utils

import (
	"context"
	"sort"

	models "github.com/jingle2008/toolkit/pkg/models"
)

type gpuHelper interface {
	ListGpuNodes() ([]models.GpuNode, error)
	ListDedicatedAIClusters() ([]models.DedicatedAICluster, error)
}

var helperFactory = func(configFile, kubeContext string) (gpuHelper, error) {
	return NewK8sHelper(configFile, kubeContext)
}

/*
LoadGpuNodes loads GPU node information from the given config file and environment.
Now accepts context.Context as the first parameter.
*/
func LoadGpuNodes(_ context.Context, configFile string, env models.Environment) (map[string][]models.GpuNode, error) {
	k8sHelper, err := helperFactory(configFile, env.GetKubeContext())
	if err != nil {
		return nil, err
	}

	nodes, err := k8sHelper.ListGpuNodes()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GpuNode)
	for _, node := range nodes {
		result[node.NodePool] = append(result[node.NodePool], node)
	}

	// sort by free GPUs
	for _, v := range result {
		sort.Slice(v, func(i, j int) bool {
			vi := v[i].Allocatable - v[i].Allocated
			vj := v[j].Allocatable - v[j].Allocated
			return vi > vj
		})
	}

	return result, nil
}

/*
LoadDedicatedAIClusters loads DedicatedAICluster information from the given config file and environment.
Now accepts context.Context as the first parameter.
*/
func LoadDedicatedAIClusters(_ context.Context, configFile string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	k8sHelper, err := helperFactory(configFile, env.GetKubeContext())
	if err != nil {
		return nil, err
	}

	dacs, err := k8sHelper.ListDedicatedAIClusters()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.DedicatedAICluster)
	for _, dac := range dacs {
		result[dac.TenantID] = append(result[dac.TenantID], dac)
	}

	for _, v := range result {
		sortKeyedItems(v)
	}

	return result, nil
}
