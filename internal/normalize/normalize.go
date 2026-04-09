package normalize

import (
	"encoding/json"
	"strings"

	"sigs.k8s.io/yaml"
)

// Top-level fields to remove entirely.
var topLevelRemove = map[string]bool{
	"status": true,
}

// Metadata fields to remove.
var metadataRemove = map[string]bool{
	"uid":                          true,
	"resourceVersion":              true,
	"generation":                   true,
	"creationTimestamp":            true,
	"deletionTimestamp":            true,
	"deletionGracePeriodSeconds":   true,
	"managedFields":                true,
	"selfLink":                     true,
}

// Annotation prefixes to strip.
var annotationPrefixRemove = []string{
	"kubectl.kubernetes.io/",
	"deployment.kubernetes.io/",
	"control-plane.alpha.kubernetes.io/",
	"pulumi.com/",
	"argocd.argoproj.io/",
}

// Normalize strips runtime-generated fields from a Kubernetes resource
// and returns canonical YAML suitable for diffing.
func Normalize(obj map[string]interface{}) string {
	out := deepCopy(obj)

	// Remove top-level runtime fields
	for key := range topLevelRemove {
		delete(out, key)
	}

	// Clean metadata
	if meta, ok := out["metadata"].(map[string]interface{}); ok {
		for key := range metadataRemove {
			delete(meta, key)
		}
		cleanAnnotations(meta)
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return ""
	}
	return string(data)
}

func cleanAnnotations(meta map[string]interface{}) {
	annotations, ok := meta["annotations"].(map[string]interface{})
	if !ok {
		return
	}

	for key := range annotations {
		for _, prefix := range annotationPrefixRemove {
			if strings.HasPrefix(key, prefix) {
				delete(annotations, key)
				break
			}
		}
	}

	if len(annotations) == 0 {
		delete(meta, "annotations")
	}
}

func deepCopy(obj map[string]interface{}) map[string]interface{} {
	data, err := json.Marshal(obj)
	if err != nil {
		return obj
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return obj
	}
	return out
}
