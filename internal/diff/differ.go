package diff

import (
	"fmt"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

// Compute returns a unified diff between two YAML strings.
// Returns empty string if they are equal.
func Compute(nameA, nameB, yamlA, yamlB string) string {
	if yamlA == yamlB {
		return ""
	}
	edits := myers.ComputeEdits(span.URIFromPath(nameA), yamlA, yamlB)
	unified := gotextdiff.ToUnified(nameA, nameB, yamlA, edits)
	return fmt.Sprint(unified)
}
