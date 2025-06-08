// Package k8s contains helpers for interacting with Kubernetes clusters and related resources.
package k8s

// TenantIDFromLabels extracts the tenant ID from a map of labels.
// Returns "UNKNOWN_TENANCY" if the label is missing or not a string.
func TenantIDFromLabels(labels map[string]interface{}) string {
	value := labels["tenancy-id"]
	if value == nil {
		return "UNKNOWN_TENANCY"
	}
	if str, ok := value.(string); ok {
		return str
	}
	return "UNKNOWN_TENANCY"
}
