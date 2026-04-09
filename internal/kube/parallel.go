package kube

import (
	"context"
	"sort"
	"sync"

	"k8s.io/client-go/dynamic"
	"github.com/ramsrib/kluster-compare/internal/diff"
	"github.com/ramsrib/kluster-compare/internal/model"
)

const maxConcurrency = 10

// FetchAndCompare fetches all registered resource types from both clusters
// concurrently and returns comparison results.
func FetchAndCompare(
	ctx context.Context,
	leftClient, rightClient dynamic.Interface,
	registry *model.Registry,
	namespaces []string,
) []model.ComparisonResult {
	types := registry.All()
	results := make([]model.ComparisonResult, len(types))
	sem := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	for i, rt := range types {
		wg.Add(1)
		go func(idx int, rt model.ResourceType) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = compareResourceType(ctx, leftClient, rightClient, rt, namespaces)
		}(i, rt)
	}
	wg.Wait()

	return results
}

// coreGroups are fetched first (higher priority).
var coreGroups = map[string]bool{
	"":                          true, // core: namespaces, services, configmaps, etc.
	"apps":                      true, // deployments, daemonsets, statefulsets
	"autoscaling":               true, // HPA
	"policy":                    true, // PDB
	"rbac.authorization.k8s.io": true, // roles, clusterroles
	"batch":                     true, // jobs, cronjobs
	"networking.k8s.io":         true, // networkpolicies, ingresses
	"scheduling.k8s.io":         true, // priorityclasses
}

// FetchAndCompareStreaming fetches resource types and calls onResult for each
// as it completes. Core K8s resources are prioritized — they get the semaphore
// first, but custom types start as soon as slots free up.
func FetchAndCompareStreaming(
	ctx context.Context,
	leftClient, rightClient dynamic.Interface,
	registry *model.Registry,
	namespaces []string,
	onResult func(index int, result model.ComparisonResult),
) {
	types := registry.All()
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// Launch core resources first — they grab semaphore slots immediately
	var customIndices []int
	for i, rt := range types {
		if coreGroups[rt.Group] {
			wg.Add(1)
			go func(idx int, rt model.ResourceType) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				onResult(idx, compareResourceType(ctx, leftClient, rightClient, rt, namespaces))
			}(i, rt)
		} else {
			customIndices = append(customIndices, i)
		}
	}

	// Launch custom resources — they queue behind core in the semaphore
	for _, i := range customIndices {
		wg.Add(1)
		go func(idx int, rt model.ResourceType) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			onResult(idx, compareResourceType(ctx, leftClient, rightClient, rt, namespaces))
		}(i, types[i])
	}

	wg.Wait()
}

func compareResourceType(
	ctx context.Context,
	leftClient, rightClient dynamic.Interface,
	rt model.ResourceType,
	namespaces []string,
) model.ComparisonResult {
	fetcher := NewDynamicFetcher(rt)

	type fetchResult struct {
		resources []model.Resource
		err       error
	}

	leftCh := make(chan fetchResult, 1)
	rightCh := make(chan fetchResult, 1)

	go func() {
		resources, err := fetcher.Fetch(ctx, leftClient, namespaces)
		leftCh <- fetchResult{resources, err}
	}()
	go func() {
		resources, err := fetcher.Fetch(ctx, rightClient, namespaces)
		rightCh <- fetchResult{resources, err}
	}()

	leftResult := <-leftCh
	rightResult := <-rightCh

	result := model.ComparisonResult{Type: rt}

	if leftResult.err != nil && rightResult.err != nil {
		result.Error = leftResult.err.Error() + "; " + rightResult.err.Error()
		return result
	}
	if leftResult.err != nil {
		result.Error = "left: " + leftResult.err.Error()
		return result
	}
	if rightResult.err != nil {
		result.Error = "right: " + rightResult.err.Error()
		return result
	}

	result.LeftCount = len(leftResult.resources)
	result.RightCount = len(rightResult.resources)

	// Build maps by key
	leftMap := make(map[string]model.Resource, len(leftResult.resources))
	for _, r := range leftResult.resources {
		leftMap[r.Key()] = r
	}
	rightMap := make(map[string]model.Resource, len(rightResult.resources))
	for _, r := range rightResult.resources {
		rightMap[r.Key()] = r
	}

	// Collect all keys
	allKeys := make(map[string]bool)
	for k := range leftMap {
		allKeys[k] = true
	}
	for k := range rightMap {
		allKeys[k] = true
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		left, inLeft := leftMap[key]
		right, inRight := rightMap[key]

		pair := model.ResourcePair{Key: key}

		switch {
		case inLeft && !inRight:
			pair.Left = &left
			pair.Status = model.StatusOnlyInLeft
		case !inLeft && inRight:
			pair.Right = &right
			pair.Status = model.StatusOnlyInRight
		default:
			pair.Left = &left
			pair.Right = &right
			diffText := diff.Compute(
				"left/"+key, "right/"+key,
				left.Normalized, right.Normalized,
			)
			if diffText == "" {
				pair.Status = model.StatusEqual
			} else {
				pair.Status = model.StatusChanged
				pair.DiffText = diffText
			}
		}

		result.Pairs = append(result.Pairs, pair)
	}

	return result
}
