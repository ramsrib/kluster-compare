package kube

import (
	"context"

	"k8s.io/client-go/dynamic"
	"github.com/ramsrib/kluster-compare/internal/model"
)

// ResourceFetcher fetches all resources of a given type from a cluster.
type ResourceFetcher interface {
	Fetch(ctx context.Context, client dynamic.Interface, namespaces []string) ([]model.Resource, error)
	Type() model.ResourceType
}
