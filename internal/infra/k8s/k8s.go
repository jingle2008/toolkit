/*
Package k8s provides helper functions for interacting with Kubernetes clusters and related resources.
*/
package k8s

import (
	"context"
	"fmt"
	"sort"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/infra/logging"
	models "github.com/jingle2008/toolkit/pkg/models"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GPUProperty is the Kubernetes resource name for GPU.
const GPUProperty corev1.ResourceName = "nvidia.com/gpu"

// NodeCondGpuUnhealthy is the condition type for unhealthy GPU nodes.
const NodeCondGpuUnhealthy corev1.NodeConditionType = "GpuUnhealthy"

/*
Helper provides helpers for interacting with Kubernetes clusters.
It manages client configuration, context switching, and provides methods for listing GPU nodes and AI clusters.
*/
type Helper struct {
	context    string
	configFile string
	config     *rest.Config

	clientset     kubernetesClient
	dynamic       dynamicClient
	clientsetFunc func(*rest.Config) (kubernetesClient, error)
	dynamicFunc   func(*rest.Config) (dynamicClient, error)
}

type kubernetesClient interface {
	CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error)
	CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error)
}

type dynamicClient interface {
	ResourceList(ctx context.Context, gvr schema.GroupVersionResource, opts v1.ListOptions) (*unstructured.UnstructuredList, error)
}

/*
NewHelper creates a new Helper using the given kubeconfig file and context.
*/
func NewHelper(configFile string, context string) (*Helper, error) {
	helper := &Helper{
		configFile:    configFile,
		clientsetFunc: defaultKubernetesClient,
		dynamicFunc:   defaultDynamicClient,
	}

	if configFile != "" && context != "" {
		err := helper.ChangeContext(context)
		if err != nil {
			return nil, fmt.Errorf("failed to change context in NewHelper: %w", err)
		}
	}

	return helper, nil
}

// Default implementations for production.
func defaultKubernetesClient(cfg *rest.Config) (kubernetesClient, error) {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return &realKubernetesClient{cs}, nil
}

type realKubernetesClient struct{ cs *kubernetes.Clientset }

func (r *realKubernetesClient) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	list, err := r.cs.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return list.Items, nil
}

func (r *realKubernetesClient) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	list, err := r.cs.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return list.Items, nil
}

func defaultDynamicClient(cfg *rest.Config) (dynamicClient, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return &realDynamicClient{dyn}, nil
}

type realDynamicClient struct{ dyn dynamic.Interface }

func (r *realDynamicClient) ResourceList(ctx context.Context, gvr schema.GroupVersionResource, opts v1.ListOptions) (*unstructured.UnstructuredList, error) {
	return r.dyn.Resource(gvr).List(ctx, opts)
}

/*
ChangeContext switches the current context of the K8sHelper to the specified context.
*/
func (k *Helper) ChangeContext(context string) error {
	if k.context == context {
		return nil
	}

	k.context = context

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: k.configFile},
		&clientcmd.ConfigOverrides{CurrentContext: k.context},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to change context: %w", err)
	}

	k.config = config
	return nil
}

/*
DefaultGPUSelectors is the default set of label selectors used to sum GPU allocations.
*/
var DefaultGPUSelectors = []string{
	"app=dummy",
	"component=predictor",
	"ome.oracle.com/trainingjob",
}

/*
ListGpuNodesWithSelectors returns a list of GpuNode objects from the current Kubernetes context.
If no selectors are provided, DefaultGPUSelectors is used.
*/
func (k *Helper) ListGpuNodesWithSelectors(ctx context.Context, selectors ...string) ([]models.GpuNode, error) {
	if len(selectors) == 0 {
		selectors = DefaultGPUSelectors
	}
	var err error
	if k.clientset == nil {
		k.clientset, err = k.clientsetFunc(k.config)
		if err != nil {
			return nil, err
		}
	}
	clientset := k.clientset

	nodes, err := clientset.CoreV1NodesList(ctx, v1.ListOptions{
		LabelSelector: "nvidia.com/gpu.present=true",
	})
	if err != nil {
		return nil, err
	}

	gpuAllocationMap := make(map[string]int64)
	for _, node := range nodes {
		gpuAllocationMap[node.Name] = 0
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, sel := range selectors {
		selCopy := sel
		eg.Go(func() error {
			err := updateGpuAllocations(egCtx, clientset, gpuAllocationMap, selCopy)
			if err != nil {
				logging.FromContext(egCtx).Errorw("updateGpuAllocations failed", "selector", selCopy, "err", err)
			}
			return nil // always return nil so all selectors run, errors are logged
		})
	}
	_ = eg.Wait()

	gpuNodes := make([]models.GpuNode, 0, len(nodes))
	for _, node := range nodes {
		allocQty := node.Status.Allocatable[GPUProperty]
		allocatable, _ := allocQty.AsInt64()
		gpuNodes = append(gpuNodes, models.GpuNode{
			Name:         node.Name,
			InstanceType: node.Labels["beta.kubernetes.io/instance-type"],
			NodePool:     node.Labels["instance-pool.name"],
			Allocatable:  int(allocatable),
			Allocated:    int(gpuAllocationMap[node.Name]),
			IsHealthy:    isNodeHealthy(node.Status.Conditions),
			IsReady:      isNodeReady(node.Status.Conditions),
		})
	}

	return gpuNodes, nil
}

// ListGpuNodes is the production version, using all selectors.
func (k *Helper) ListGpuNodes(ctx context.Context) ([]models.GpuNode, error) {
	return k.ListGpuNodesWithSelectors(ctx)
}

func updateGpuAllocations(ctx context.Context, clientset kubernetesClient,
	gpuAllocationMap map[string]int64, label string,
) error {
	pods, err := clientset.CoreV1PodsList(ctx, "", v1.ListOptions{
		LabelSelector: label,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for selector %q in updateGpuAllocations: %w", label, err)
	}

	for _, pod := range pods {
		if _, ok := gpuAllocationMap[pod.Spec.NodeName]; ok {
			gpuAllocationMap[pod.Spec.NodeName] += calculatePodGPUs(&pod)
		}
	}

	return nil
}

func calculatePodGPUs(pod *corev1.Pod) int64 {
	var total int64
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Requests[GPUProperty]; ok {
			total += val.Value()
		}
	}
	return total
}

func isNodeHealthy(conditions []corev1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == NodeCondGpuUnhealthy {
			return condition.Status == corev1.ConditionFalse
		}
	}

	return false
}

func isNodeReady(conditions []corev1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

/*
ListDedicatedAIClusters returns all DedicatedAICluster resources from both v1alpha1 and v1beta1 CRDs.
*/
func (k *Helper) ListDedicatedAIClusters(ctx context.Context) ([]models.DedicatedAICluster, error) {
	var err error
	if k.dynamic == nil {
		k.dynamic, err = k.dynamicFunc(k.config)
		if err != nil {
			return nil, err
		}
	}
	dyn := k.dynamic

	v1Clusters, err := k.listDedicatedAIClustersV1(ctx, dyn)
	if err != nil {
		return nil, err
	}
	v2Clusters, err := k.listDedicatedAIClustersV2(ctx, dyn)
	if err != nil {
		return nil, err
	}
	return append(v1Clusters, v2Clusters...), nil
}

// listDedicatedAIClustersGeneric fetches DedicatedAIClusters using a GVR and extractor.
func listDedicatedAIClustersGeneric(
	ctx context.Context,
	dyn dynamicClient,
	gvr schema.GroupVersionResource,
	extract func(item unstructured.Unstructured) models.DedicatedAICluster,
) ([]models.DedicatedAICluster, error) {
	list, err := dyn.ResourceList(ctx, gvr, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	dacs := make([]models.DedicatedAICluster, 0, len(list.Items))
	for _, item := range list.Items {
		dacs = append(dacs, extract(item))
	}
	return dacs, nil
}

// listDedicatedAIClustersV1 fetches DedicatedAIClusters from v1alpha1 CRD
func (k *Helper) listDedicatedAIClustersV1(ctx context.Context, dyn dynamicClient) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.oracle.com",
		Version:  "v1alpha1",
		Resource: "dedicatedaiclusters",
	}
	return listDedicatedAIClustersGeneric(ctx, dyn, gvr, func(item unstructured.Unstructured) models.DedicatedAICluster {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")

		dacType, _ := spec["type"].(string)
		unitShape, _ := spec["unitShape"].(string)
		size, _ := spec["size"].(int64)

		tenantID := "missing"
		if hasLabels {
			tenantID = TenantIDFromLabels(labels)
		}

		statusStr, _ := status["status"].(string)
		if statusStr == "" {
			statusStr = "pending"
		}

		return models.DedicatedAICluster{
			Name:      name,
			Type:      dacType,
			UnitShape: unitShape,
			Size:      int(size),
			Status:    statusStr,
			TenantID:  tenantID,
		}
	})
}

// listDedicatedAIClustersV2 fetches DedicatedAIClusters from v1beta1 CRD
func (k *Helper) listDedicatedAIClustersV2(ctx context.Context, dyn dynamicClient) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "dedicatedaiclusters",
	}
	return listDedicatedAIClustersGeneric(ctx, dyn, gvr, func(item unstructured.Unstructured) models.DedicatedAICluster {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")

		profile, _ := spec["profile"].(string)
		count, _ := spec["count"].(int64)

		tenantID := "missing"
		if hasLabels {
			tenantID = TenantIDFromLabels(labels)
		}

		dacLifecycleState, _ := status["dacLifecycleState"].(string)
		statusStr := dacLifecycleState
		if statusStr == "" {
			statusStr = "pending"
		}

		return models.DedicatedAICluster{
			Name:     name,
			Profile:  profile,
			Size:     int(count),
			Status:   statusStr,
			TenantID: tenantID,
		}
	})
}

/*
GpuNodeLister is an interface for listing GPU nodes.
*/
type GpuNodeLister interface {
	ListGpuNodes(ctx context.Context) ([]models.GpuNode, error)
}

/*
LoadGpuNodes loads GPU node information using the provided GpuNodeLister.
*/
func LoadGpuNodes(ctx context.Context, lister GpuNodeLister) (map[string][]models.GpuNode, error) {
	nodes, err := lister.ListGpuNodes(ctx)
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
DedicatedAIClusterLister is an interface for listing DedicatedAIClusters.
*/
type DedicatedAIClusterLister interface {
	ListDedicatedAIClusters(ctx context.Context) ([]models.DedicatedAICluster, error)
}

/*
LoadDedicatedAIClusters loads DedicatedAICluster information using the provided DedicatedAIClusterLister.
*/
func LoadDedicatedAIClusters(ctx context.Context, lister DedicatedAIClusterLister) (map[string][]models.DedicatedAICluster, error) {
	dacs, err := lister.ListDedicatedAIClusters(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.DedicatedAICluster)
	for _, dac := range dacs {
		result[dac.TenantID] = append(result[dac.TenantID], dac)
	}

	for _, v := range result {
		collections.SortKeyedItems(v)
	}

	return result, nil
}
