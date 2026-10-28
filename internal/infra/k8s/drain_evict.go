// Drain eviction: what blocks a drain, and evicting the pods on the node.

package k8s

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"

	"github.com/jingle2008/toolkit/internal/logging"
)

const (
	fineTuneLabel       = "finetunedmodel-name"
	replicationJobLabel = "replicationJob"
)

// drainBlockerLabels mark pods whose eviction loses work in flight; a
// node running any of them must not be drained.
var drainBlockerLabels = []string{fineTuneLabel, trainingLabelV2, replicationJobLabel}

// Eviction seams for tests. checkEvictionSupport is an API discovery
// call, so it runs once per eviction round, not once per pod.
var (
	checkEvictionSupport = drain.CheckEvictionSupport
	// evictPod evicts one pod via the eviction API group gv, deleting it
	// instead where the cluster lacks the eviction API (gv empty).
	evictPod = func(helper *drain.Helper, pod corev1.Pod, gv schema.GroupVersion) error {
		if gv.Empty() {
			return helper.DeletePod(pod)
		}
		return helper.EvictPod(pod, gv)
	}
)

// evictRound evicts every pod not already terminating. A refused eviction
// (e.g. 429 from a PodDisruptionBudget) is retried next round until the
// phase times out.
func (d *drainer) evictRound(ctx context.Context, helper *drain.Helper, pods []corev1.Pod, gv schema.GroupVersion) {
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue // already terminating
		}
		name := pod.Namespace + "/" + pod.Name
		err := evictPod(helper, pod, gv)
		switch {
		case err == nil:
			note(ctx, false, "drain: evicted", "pod", name)
		case apierrors.IsNotFound(err):
		case d.shouldNote("deferred:"+name, err.Error(), drainProgressEvery):
			note(ctx, false, "drain: eviction deferred, will retry", "pod", name, "error", err)
		}
	}
}

func (d *drainer) evictHelper(ctx context.Context) *drain.Helper {
	logger := logging.FromContext(ctx)
	return &drain.Helper{
		Ctx:                 ctx,
		Client:              d.clientset,
		Out:                 logWriter{logger},
		ErrOut:              logWriter{logger},
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1, // Use pod's termination grace period
	}
}

func podsOnNode(ctx context.Context, clientset kubernetes.Interface, nodeName string) ([]corev1.Pod, error) {
	list, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods on node %s: %w", nodeName, err)
	}
	return list.Items, nil
}

// podActive reports whether the pod still holds the node: not yet
// completed or failed.
func podActive(pod *corev1.Pod) bool {
	return pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed
}

// emptyDirPods names the pods that mount an emptyDir volume.
func emptyDirPods(pods []corev1.Pod) []string {
	var names []string
	for i := range pods {
		for _, vol := range pods[i].Spec.Volumes {
			if vol.EmptyDir != nil {
				names = append(names, pods[i].Namespace+"/"+pods[i].Name)
				break
			}
		}
	}
	return names
}

func podNames(pods []corev1.Pod) []string {
	names := make([]string, len(pods))
	for i := range pods {
		names[i] = pods[i].Namespace + "/" + pods[i].Name
	}
	return names
}

func checkDrainBlockers(pods []corev1.Pod) error {
	var blockers []string
	for i := range pods {
		pod := &pods[i]
		if !podActive(pod) {
			continue
		}
		for _, label := range drainBlockerLabels {
			if _, ok := pod.Labels[label]; ok {
				blockers = append(blockers, fmt.Sprintf("%s/%s (%s)", pod.Namespace, pod.Name, label))
				break
			}
		}
	}
	if len(blockers) > 0 {
		return fmt.Errorf("node runs pods that must not be drained: %s", strings.Join(blockers, ", "))
	}
	return nil
}
