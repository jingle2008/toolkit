/*
Package utils provides helper functions for interacting with Kubernetes clusters and related resources.
*/
package utils

import (
	"context"
	"log"

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
}

/*
NewK8sHelper creates a new K8sHelper using the given kubeconfig file and context.
*/
func NewK8sHelper(configFile string, context string) (*K8sHelper, error) {
	helper := &K8sHelper{
		configFile: configFile,
	}

	if configFile != "" && context != "" {
		err := helper.ChangeContext(context)
		if err != nil {
			return nil, err
		}
	}

	return helper, nil
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
		return err
	}

	k.config = config
	return nil
}

/*
ListGpuNodes returns a list of GpuNode objects from the current Kubernetes context.
*/
func (k *K8sHelper) ListGpuNodes() ([]models.GpuNode, error) {
	clientset, err := kubernetes.NewForConfig(k.config)
	if err != nil {
		return nil, err
	}

	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
		LabelSelector: "nvidia.com/gpu.present=true",
	})
	if err != nil {
		return nil, err
	}

	gpuAllocationMap := make(map[string]int64)
	for _, node := range nodeList.Items {
		gpuAllocationMap[node.Name] = 0
	}

	// GPU with no workload
	if err := updateGpuAllocations(clientset, gpuAllocationMap, "app=dummy"); err != nil {
		// WARN: updateGpuAllocations dummy
		log.Printf("WARN: updateGpuAllocations dummy: %v", err)
	}
	// GPU with serving workload
	if err := updateGpuAllocations(clientset, gpuAllocationMap, "component=predictor"); err != nil {
		// WARN: updateGpuAllocations predictor
		log.Printf("WARN: updateGpuAllocations predictor: %v", err)
	}
	// GPU with training workload
	if err := updateGpuAllocations(clientset, gpuAllocationMap, "ome.oracle.com/trainingjob"); err != nil {
		// WARN: updateGpuAllocations trainingjob
		log.Printf("WARN: updateGpuAllocations trainingjob: %v", err)
	}

	gpuNodes := make([]models.GpuNode, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
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

func updateGpuAllocations(clientset *kubernetes.Clientset,
	gpuAllocationMap map[string]int64, label string,
) error {
	// Use a field selector to get only pods with GPU requests
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), v1.ListOptions{
		LabelSelector: label,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return err
	}

	// Process pods
	for _, pod := range pods.Items {
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
func (k *K8sHelper) ListDedicatedAIClusters() ([]models.DedicatedAICluster, error) {
	dyn, err := dynamic.NewForConfig(k.config)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()

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
func (k *K8sHelper) listDedicatedAIClustersV1(ctx context.Context, dyn dynamic.Interface) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.oracle.com",
		Version:  "v1alpha1",
		Resource: "dedicatedaiclusters",
	}
	list, err := dyn.Resource(gvr).List(ctx, v1.ListOptions{})
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
func (k *K8sHelper) listDedicatedAIClustersV2(ctx context.Context, dyn dynamic.Interface) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "dedicatedaiclusters",
	}
	list, err := dyn.Resource(gvr).List(ctx, v1.ListOptions{})
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

// tenantIDFromLabels extracts the tenancy-id from a labels map
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
