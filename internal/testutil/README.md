# testutil

This package contains shared test helpers, fake clients, and fixtures for use across unit and integration tests.

- All code here should be test-only (never imported by production code).
- Use the `_test.go` suffix for all files to ensure they are excluded from production builds.
- Place generic fakes and golden-file helpers here to avoid duplication across packages.

## Kubernetes Testing

For Kubernetes-related tests, use the provided fake client helpers:

- `NewFakeClient(objs ...runtime.Object) *fake.Clientset`: creates a fake Kubernetes client pre-loaded with objects.
- `NewFakeKubernetesClientAdapter(clientset *fake.Clientset) TestKubernetesClient`: returns a test adapter implementing the minimal interface for K8sHelper tests.

Example usage:

```go
import "yourmodule/internal/testutil"

pod := &corev1.Pod{ /* ... */ }
client := testutil.NewFakeClient(pod)
adapter := testutil.NewFakeKubernetesClientAdapter(client)
// Use adapter in your K8sHelper tests as a TestKubernetesClient
```

## Usage

Import as `testutil` in your test files:

```go
import "yourmodule/internal/testutil"
