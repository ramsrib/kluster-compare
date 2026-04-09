package normalize

import (
	"strings"
	"testing"
)

func TestNormalize_StripsRuntimeFields(t *testing.T) {
	obj := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":              "api-server",
			"namespace":         "app-namespace",
			"uid":               "abc-123",
			"resourceVersion":   "999",
			"generation":        int64(5),
			"creationTimestamp":  "2024-01-01T00:00:00Z",
			"managedFields":     []interface{}{"something"},
			"selfLink":          "/apis/apps/v1/namespaces/default/deployments/api",
			"labels": map[string]interface{}{
				"app": "api-server",
			},
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "big json blob",
				"deployment.kubernetes.io/revision":                "3",
				"pulumi.com/autonamed":                             "true",
				"argocd.argoproj.io/sync-wave":                     "1",
				"custom.io/important":                              "keep-me",
			},
		},
		"spec": map[string]interface{}{
			"replicas": int64(3),
		},
		"status": map[string]interface{}{
			"readyReplicas": int64(3),
		},
	}

	result := Normalize(obj)

	// Should NOT contain stripped fields
	for _, shouldNotContain := range []string{
		"uid:",
		"resourceVersion:",
		"generation:",
		"creationTimestamp:",
		"managedFields:",
		"selfLink:",
		"status:",
		"readyReplicas:",
		"last-applied-configuration",
		"pulumi.com",
		"argocd.argoproj.io",
		"deployment.kubernetes.io/revision",
	} {
		if strings.Contains(result, shouldNotContain) {
			t.Errorf("normalized output should not contain %q, got:\n%s", shouldNotContain, result)
		}
	}

	// Should contain kept fields
	for _, shouldContain := range []string{
		"apiVersion: apps/v1",
		"kind: Deployment",
		"name: api-server",
		"namespace: app-namespace",
		"app: api-server",
		"custom.io/important: keep-me",
		"replicas: 3",
	} {
		if !strings.Contains(result, shouldContain) {
			t.Errorf("normalized output should contain %q, got:\n%s", shouldContain, result)
		}
	}
}

func TestNormalize_EmptyAnnotationsRemoved(t *testing.T) {
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": "test",
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "blob",
			},
		},
	}

	result := Normalize(obj)
	if strings.Contains(result, "annotations:") {
		t.Errorf("empty annotations should be removed, got:\n%s", result)
	}
}
