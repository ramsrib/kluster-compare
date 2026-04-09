package diff

import (
	"strings"
	"testing"
)

func TestCompute_Equal(t *testing.T) {
	yaml := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n"
	result := Compute("left", "right", yaml, yaml)
	if result != "" {
		t.Errorf("expected empty diff for equal inputs, got: %s", result)
	}
}

func TestCompute_Different(t *testing.T) {
	yamlA := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  replicas: 3\n  image: api:v1.0\n"
	yamlB := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  replicas: 5\n  image: api:v1.0\n"

	result := Compute("left/deploy", "right/deploy", yamlA, yamlB)

	if result == "" {
		t.Fatal("expected non-empty diff for different inputs")
	}
	if !strings.Contains(result, "-  replicas: 3") {
		t.Errorf("diff should show removal of replicas: 3, got:\n%s", result)
	}
	if !strings.Contains(result, "+  replicas: 5") {
		t.Errorf("diff should show addition of replicas: 5, got:\n%s", result)
	}
}
