/*
Package k8s provides helper functions for interacting with Kubernetes clusters and related resources.
*/
package k8s

import (
	"context"
	"fmt"
	"sort"

	"github.com/jingle2008/toolkit/internal/infra/logging"
	models "github.com/jingle2008/toolkit/pkg/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GPUProperty is the Kubernetes resource name for GPU.
const GPUProperty = "nvidia.com/gpu"

// K8sHelper provides helpers for interacting with Kubernetes clusters.
type K8sHelper struct {
	context    string
	configFile string
	config     *rest.Config

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
NewK8sHelper creates a new K8sHelper using the given kubeconfig file and context.
*/
func NewK8sHelper(configFile string, context string) (*K8sHelper, error) {
	helper := &K8sHelper{
		configFile:    configFile,
		clientsetFunc: defaultKubernetesClient,
		dynamicFunc:   defaultDynamicClient,
	}

	if configFile != "" && context != "" {
		err := helper.ChangeContext(context)
		if err != nil {
			return nil, fmt.Errorf("failed to change context in NewK8sHelper: %w", err)
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
func (k *K8sHelper) ChangeContext(context string) error {
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
ListGpuNodesWithSelectors returns a list of GpuNode objects from the current Kubernetes context.
By default, it sums allocations for three label selectors. For testability, you can override the selectors.
*/
func (k *K8sHelper) ListGpuNodesWithSelectors(ctx context.Context, selectors ...string) ([]models.GpuNode, error) {
	clientset, err := k.clientsetFunc(k.config)
	if err != nil {
		return nil, err
	}

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

	for _, sel := range selectors {
		if err := updateGpuAllocations(ctx, clientset, gpuAllocationMap, sel); err != nil {
			logging.LoggerFromCtx(ctx).Errorw("updateGpuAllocations failed", "selector", sel, "err", err)
		}
	}

	gpuNodes := make([]models.GpuNode, 0, len(nodes))
	for _, node := range nodes {
		allocatable, _ := node.Status.Allocatable.Name(GPUProperty, resource.DecimalSI).AsInt64()
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
func (k *K8sHelper) ListGpuNodes(ctx context.Context) ([]models.GpuNode, error) {
	return k.ListGpuNodesWithSelectors(ctx, "app=dummy", "component=predictor", "ome.oracle.com/trainingjob")
}

func updateGpuAllocations(ctx context.Context, clientset kubernetesClient,
	gpuAllocationMap map[string]int64, label string,
) error {
	pods, err := clientset.CoreV1PodsList(ctx, "", v1.ListOptions{
		LabelSelector: label,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods in updateGpuAllocations: %w", err)
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
		if condition.Type == corev1.NodeConditionType("GpuUnhealthy") {
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
func (k *K8sHelper) ListDedicatedAIClusters(ctx context.Context) ([]models.DedicatedAICluster, error) {
	dyn, err := k.dynamicFunc(k.config)
	if err != nil {
		return nil, err
	}

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

// listDedicatedAIClustersV1 fetches DedicatedAIClusters from v1alpha1 CRD
func (k *K8sHelper) listDedicatedAIClustersV1(ctx context.Context, dyn dynamicClient) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.oracle.com",
		Version:  "v1alpha1",
		Resource: "dedicatedaiclusters",
	}
	list, err := dyn.ResourceList(ctx, gvr, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var dacs []models.DedicatedAICluster
	for _, item := range list.Items {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")

		dacType, _ := spec["type"].(string)
		unitShape, _ := spec["unitShape"].(string)
		size, _ := spec["size"].(int64)

		tenantID := "missing"
		if hasLabels {
			tenantID = tenantIDFromLabels(labels)
		}

		statusStr, _ := status["status"].(string)
		if statusStr == "" {
			statusStr = "pending"
		}

		dacs = append(dacs, models.DedicatedAICluster{
			Name:      name,
			Type:      dacType,
			UnitShape: unitShape,
			Size:      int(size),
			Status:    statusStr,
			TenantID:  tenantID,
		})
	}
	return dacs, nil
}

// listDedicatedAIClustersV2 fetches DedicatedAIClusters from v1beta1 CRD
func (k *K8sHelper) listDedicatedAIClustersV2(ctx context.Context, dyn dynamicClient) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "dedicatedaiclusters",
	}
	list, err := dyn.ResourceList(ctx, gvr, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var dacs []models.DedicatedAICluster
	for _, item := range list.Items {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")

		profile, _ := spec["profile"].(string)
		count, _ := spec["count"].(int64)

		tenantID := "missing"
		if hasLabels {
			tenantID = tenantIDFromLabels(labels)
		}

		dacLifecycleState, _ := status["dacLifecycleState"].(string)
		statusStr := dacLifecycleState
		if statusStr == "" {
			statusStr = "pending"
		}

		dacs = append(dacs, models.DedicatedAICluster{
			Name:     name,
			Profile:  profile,
			Size:     int(count),
			Status:   statusStr,
			TenantID: tenantID,
		})
	}
	return dacs, nil
}

func tenantIDFromLabels(labels map[string]interface{}) string {
	value := labels["tenancy-id"]
	if value == nil {
		return "UNKNOWN_TENANCY"
	}
	if str, ok := value.(string); ok {
		return str
	}
	return "UNKNOWN_TENANCY"
}

// gpuHelper interface and helperFactory for compatibility with previous utils API.
type gpuHelper interface {
	ListGpuNodes(ctx context.Context) ([]models.GpuNode, error)
	ListDedicatedAIClusters(ctx context.Context) ([]models.DedicatedAICluster, error)
}

var helperFactory = func(configFile, kubeContext string) (gpuHelper, error) {
	return NewK8sHelper(configFile, kubeContext)
}

/*
LoadGpuNodes loads GPU node information from the given config file and environment.
Now accepts context.Context as the first parameter.
*/
func LoadGpuNodes(ctx context.Context, configFile string, env models.Environment) (map[string][]models.GpuNode, error) {
	k8sHelper, err := helperFactory(configFile, env.GetKubeContext())
	if err != nil {
		return nil, err
	}

	nodes, err := k8sHelper.ListGpuNodes(ctx)
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
func LoadDedicatedAIClusters(ctx context.Context, configFile string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	k8sHelper, err := helperFactory(configFile, env.GetKubeContext())
	if err != nil {
		return nil, err
	}

	dacs, err := k8sHelper.ListDedicatedAIClusters(ctx)
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

// sortKeyedItems sorts a slice of KeyedItem by key.
func sortKeyedItems[T interface{ GetKey() string }](items []T) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetKey() < items[j].GetKey()
	})
}
