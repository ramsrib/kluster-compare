package model

// ResourceType describes a kind of Kubernetes resource that can be compared.
type ResourceType struct {
	Name       string // Human-readable name, e.g. "Deployments"
	Group      string // API group, e.g. "apps" or "" for core
	Version    string // API version, e.g. "v1"
	Resource   string // Plural resource name, e.g. "deployments"
	Namespaced bool   // Whether the resource is namespaced
}

// Resource is a single Kubernetes resource fetched from a cluster.
type Resource struct {
	Name       string
	Namespace  string // Empty for cluster-scoped resources
	Type       ResourceType
	Raw        map[string]interface{}
	Normalized string // YAML after normalization
}

// Key returns a unique identifier for matching resources across clusters.
func (r Resource) Key() string {
	if r.Namespace == "" {
		return r.Name
	}
	return r.Namespace + "/" + r.Name
}

// DiffStatus represents the comparison status of a resource.
type DiffStatus int

const (
	StatusEqual DiffStatus = iota
	StatusChanged
	StatusOnlyInLeft
	StatusOnlyInRight
)

func (s DiffStatus) String() string {
	switch s {
	case StatusEqual:
		return "EQUAL"
	case StatusChanged:
		return "CHANGED"
	case StatusOnlyInLeft:
		return "ONLY IN LEFT"
	case StatusOnlyInRight:
		return "ONLY IN RIGHT"
	default:
		return "UNKNOWN"
	}
}

// ResourcePair holds matched resources from both clusters.
type ResourcePair struct {
	Key      string
	Left     *Resource // nil if only in right
	Right    *Resource // nil if only in left
	Status   DiffStatus
	DiffText string // Unified diff (empty if equal or one-sided)
}

// ComparisonResult holds the comparison between two clusters for one ResourceType.
type ComparisonResult struct {
	Type       ResourceType
	Pairs      []ResourcePair
	Error      string // Non-empty if fetching failed for this type
	LeftCount  int
	RightCount int
	Loading    bool // True while still being fetched
}

// Counts returns summary counts for this comparison.
func (c ComparisonResult) Counts() (equal, changed, onlyLeft, onlyRight int) {
	for _, p := range c.Pairs {
		switch p.Status {
		case StatusEqual:
			equal++
		case StatusChanged:
			changed++
		case StatusOnlyInLeft:
			onlyLeft++
		case StatusOnlyInRight:
			onlyRight++
		}
	}
	return
}
