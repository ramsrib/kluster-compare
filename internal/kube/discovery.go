package kube

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/ramsrib/kluster-compare/internal/model"
)

// skipGroups are API groups that produce noise and aren't useful to diff.
var skipGroups = map[string]bool{
	"events.k8s.io":                    true,
	"authentication.k8s.io":            true,
	"authorization.k8s.io":             true,
	"coordination.k8s.io":              true,
	"discovery.k8s.io":                 true,
	"flowcontrol.apiserver.k8s.io":     true,
	"node.k8s.io":                      true,
	"apiregistration.k8s.io":           true,
	"admissionregistration.k8s.io":     true,
	"apiextensions.k8s.io":             true,
	"storage.k8s.io":                   true,
	"resource.k8s.io":                  true,
	"internal.apiserver.k8s.io":        true,
	"certificates.k8s.io":              true,
	"storagemigration.k8s.io":          true,
}

// skipResources are specific resources that are too noisy or ephemeral.
var skipResources = map[string]bool{
	"events":                  true,
	"endpoints":               true,
	"pods":                    true,
	"replicasets":             true,
	"controllerrevisions":     true,
	"endpointslices":          true,
	"leases":                  true,
	"nodes":                   true,
	"secrets":                 true,
}

// DiscoverResourceTypes queries the API server for all available resource types
// and returns them as a Registry.
func DiscoverResourceTypes(kubeconfigPath, contextA, contextB string) (*model.Registry, error) {
	typesA, err := discoverFromContext(kubeconfigPath, contextA)
	if err != nil {
		return nil, fmt.Errorf("discover from %s: %w", contextA, err)
	}
	typesB, err := discoverFromContext(kubeconfigPath, contextB)
	if err != nil {
		return nil, fmt.Errorf("discover from %s: %w", contextB, err)
	}

	// Union both sets, keyed by group/version/resource
	merged := make(map[string]model.ResourceType)
	for _, rt := range typesA {
		merged[rt.Group+"/"+rt.Version+"/"+rt.Resource] = rt
	}
	for _, rt := range typesB {
		merged[rt.Group+"/"+rt.Version+"/"+rt.Resource] = rt
	}

	// Sort by name for stable ordering
	sorted := make([]model.ResourceType, 0, len(merged))
	for _, rt := range merged {
		sorted = append(sorted, rt)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	reg := &model.Registry{}
	for _, rt := range sorted {
		reg.Register(rt)
	}
	return reg, nil
}

func discoverFromContext(kubeconfigPath, contextName string) ([]model.ResourceType, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	_, resourceLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		// Partial results are common (some groups may fail), continue with what we have
		if resourceLists == nil {
			return nil, err
		}
	}

	var types []model.ResourceType
	for _, list := range resourceLists {
		gv := list.GroupVersion
		group, version := parseGroupVersion(gv)

		if skipGroups[group] {
			continue
		}

		for _, r := range list.APIResources {
			// Skip subresources (e.g. pods/log, deployments/scale)
			if strings.Contains(r.Name, "/") {
				continue
			}
			if skipResources[r.Name] {
				continue
			}
			// Only include resources we can list
			if !containsVerb(r.Verbs, "list") {
				continue
			}

			types = append(types, model.ResourceType{
				Name:       humanName(r.Name),
				Group:      group,
				Version:    version,
				Resource:   r.Name,
				Namespaced: r.Namespaced,
			})
		}
	}

	return types, nil
}

func parseGroupVersion(gv string) (string, string) {
	parts := strings.SplitN(gv, "/", 2)
	if len(parts) == 1 {
		// Core group: "v1"
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func containsVerb(verbs []string, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// humanName converts a plural resource name to a human-readable title.
// e.g. "deployments" -> "Deployments", "horizontalpodautoscalers" -> "HorizontalPodAutoscalers"
func humanName(resource string) string {
	if len(resource) == 0 {
		return resource
	}
	return strings.ToUpper(resource[:1]) + resource[1:]
}
