# Kubernetes Client Fakes & Testing Patterns

This document describes recommended patterns for testing code that depends on Kubernetes clients in the toolkit.

## Use Interfaces, Not Concrete Types

- Always depend on `kubernetes.Interface` and `dynamic.Interface` (from `client-go`), not concrete clientset types.
- This allows you to use `k8s.io/client-go/kubernetes/fake` and `k8s.io/client-go/dynamic/fake` in tests.

## Example: Using the Fake Clientset

```go
import (
    "context"
    "testing"

    "k8s.io/client-go/kubernetes/fake"
    v1 "k8s.io/api/core/v1"
)

func TestListNodes(t *testing.T) {
    client := fake.NewSimpleClientset(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
    nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
    if err != nil {
        t.Fatal(err)
    }
    if len(nodes.Items) != 1 {
        t.Errorf("expected 1 node, got %d", len(nodes.Items))
    }
}
```

## Pattern: Dependency Injection

- Pass the interface (not the constructor) into your functions or structs.
- Example:

```go
type NodeLister struct {
    client kubernetes.Interface
}

func (n *NodeLister) ListNodeNames(ctx context.Context) ([]string, error) {
    nodes, err := n.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, err
    }
    var names []string
    for _, node := range nodes.Items {
        names = append(names, node.Name)
    }
    return names, nil
}
```

## Pattern: Table-Driven Tests

- Use table-driven tests to cover multiple scenarios with different fake client setups.

## Pattern: Use go:generate for Mocks

- For more complex interfaces, use tools like [counterfeiter](https://github.com/maxbrunsfeld/counterfeiter) to generate mocks.

## See Also

- [client-go testing docs](https://pkg.go.dev/k8s.io/client-go/kubernetes/fake)
- [counterfeiter](https://github.com/maxbrunsfeld/counterfeiter)
