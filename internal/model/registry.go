package model

// Registry holds all known resource types.
type Registry struct {
	types []ResourceType
}

// DefaultRegistry returns a registry with the initial set of resource types.
func DefaultRegistry() *Registry {
	return &Registry{
		types: []ResourceType{
			// Core
			{Name: "Namespaces", Group: "", Version: "v1", Resource: "namespaces", Namespaced: false},
			{Name: "Deployments", Group: "apps", Version: "v1", Resource: "deployments", Namespaced: true},
			{Name: "Services", Group: "", Version: "v1", Resource: "services", Namespaced: true},
			{Name: "DaemonSets", Group: "apps", Version: "v1", Resource: "daemonsets", Namespaced: true},
			{Name: "StatefulSets", Group: "apps", Version: "v1", Resource: "statefulsets", Namespaced: true},
			{Name: "ConfigMaps", Group: "", Version: "v1", Resource: "configmaps", Namespaced: true},
			{Name: "ServiceAccounts", Group: "", Version: "v1", Resource: "serviceaccounts", Namespaced: true},
			// RBAC
			{Name: "ClusterRoles", Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles", Namespaced: false},
			{Name: "ClusterRoleBindings", Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings", Namespaced: false},
			{Name: "Roles", Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles", Namespaced: true},
			{Name: "RoleBindings", Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings", Namespaced: true},
			// Autoscaling
			{Name: "HPA", Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers", Namespaced: true},
			// Policy
			{Name: "PDB", Group: "policy", Version: "v1", Resource: "poddisruptionbudgets", Namespaced: true},
			// Scheduling
			{Name: "PriorityClasses", Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses", Namespaced: false},
			// Networking
			{Name: "Ingresses", Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Namespaced: true},
			{Name: "NetworkPolicies", Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies", Namespaced: true},
			{Name: "IngressClasses", Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses", Namespaced: false},
			// Batch
			{Name: "Jobs", Group: "batch", Version: "v1", Resource: "jobs", Namespaced: true},
			{Name: "CronJobs", Group: "batch", Version: "v1", Resource: "cronjobs", Namespaced: true},
		},
	}
}

// Register adds a new resource type.
func (r *Registry) Register(rt ResourceType) {
	r.types = append(r.types, rt)
}

// All returns all registered resource types.
func (r *Registry) All() []ResourceType {
	return r.types
}

// Filter returns a new registry with only the named resource types.
func (r *Registry) Filter(names []string) *Registry {
	if len(names) == 0 {
		return r
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	filtered := &Registry{}
	for _, rt := range r.types {
		if nameSet[rt.Name] {
			filtered.types = append(filtered.types, rt)
		}
	}
	return filtered
}
