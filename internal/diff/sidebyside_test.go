package diff

import (
	"testing"
)

func TestSideBySide_PairsLines(t *testing.T) {
	yamlA := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  replicas: 3\n  image: api:v1.0\n"
	yamlB := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  replicas: 5\n  image: api:v2.0\n"

	unified := Compute("left", "right", yamlA, yamlB)
	lines := SideBySide(unified)

	if len(lines) == 0 {
		t.Fatal("expected side-by-side lines, got none")
	}

	// Should have at least one hunk header
	hasHunk := false
	hasChanged := false
	hasEqual := false
	for _, l := range lines {
		switch l.Kind {
		case LineHunk:
			hasHunk = true
		case LineChanged:
			hasChanged = true
		case LineEqual:
			hasEqual = true
		}
	}

	if !hasHunk {
		t.Error("expected at least one hunk header line")
	}
	if !hasChanged {
		t.Error("expected at least one changed line")
	}
	if !hasEqual {
		t.Error("expected at least one equal/context line")
	}

	// Verify changed lines pair correctly
	for _, l := range lines {
		if l.Kind == LineChanged {
			if l.Left == "" || l.Right == "" {
				t.Errorf("changed line should have both left and right, got left=%q right=%q", l.Left, l.Right)
			}
		}
	}
}
