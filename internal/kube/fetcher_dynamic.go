package kube

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"github.com/ramsrib/kluster-compare/internal/model"
	"github.com/ramsrib/kluster-compare/internal/normalize"
)

// DynamicFetcher fetches any resource type using the dynamic client.
type DynamicFetcher struct {
	resourceType model.ResourceType
}

func NewDynamicFetcher(rt model.ResourceType) *DynamicFetcher {
	return &DynamicFetcher{resourceType: rt}
}

func (f *DynamicFetcher) Type() model.ResourceType {
	return f.resourceType
}

func (f *DynamicFetcher) Fetch(ctx context.Context, client dynamic.Interface, namespaces []string) ([]model.Resource, error) {
	gvr := schema.GroupVersionResource{
		Group:    f.resourceType.Group,
		Version:  f.resourceType.Version,
		Resource: f.resourceType.Resource,
	}

	var items []unstructured.Unstructured

	if !f.resourceType.Namespaced {
		list, err := client.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			if isIgnorableError(err) {
				return nil, nil
			}
			return nil, err
		}
		items = list.Items
	} else if len(namespaces) > 0 {
		for _, ns := range namespaces {
			list, err := client.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				if isIgnorableError(err) {
					continue
				}
				return nil, err
			}
			items = append(items, list.Items...)
		}
	} else {
		list, err := client.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		if err != nil {
			if isIgnorableError(err) {
				return nil, nil
			}
			return nil, err
		}
		items = list.Items
	}

	resources := make([]model.Resource, 0, len(items))
	for _, item := range items {
		r := model.Resource{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Type:      f.resourceType,
			Raw:       item.Object,
		}
		r.Normalized = normalize.Normalize(item.Object)
		resources = append(resources, r)
	}

	return resources, nil
}

// isIgnorableError returns true for errors that mean "this resource type doesn't exist here"
// (CRD not installed, API not available) or RBAC denied.
func isIgnorableError(err error) bool {
	return apierrors.IsNotFound(err) ||
		apierrors.IsForbidden(err) ||
		apierrors.IsMethodNotSupported(err)
}
