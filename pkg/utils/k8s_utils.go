package utils

import (
	"sort"

	models "github.com/jingle2008/toolkit/pkg/models"
)

/*
LoadGpuNodes loads GPU node information from the given config file and environment.
*/
func LoadGpuNodes(configFile string, env models.Environment) (map[string][]models.GpuNode, error) {
	k8sHelper, err := NewK8sHelper(configFile, env.GetKubeContext())
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
*/
func LoadDedicatedAIClusters(configFile string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	k8sHelper, err := NewK8sHelper(configFile, env.GetKubeContext())
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
