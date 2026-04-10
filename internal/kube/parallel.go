package kube

import (
	"context"
	"sort"
	"strings"
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

	// Pass 1: exact match
	matchedLeft := make(map[string]bool)
	matchedRight := make(map[string]bool)

	// Collect exact matches first
	for key, left := range leftMap {
		if right, ok := rightMap[key]; ok {
			matchedLeft[key] = true
			matchedRight[key] = true
			pair := makePair(key, &left, &right)
			result.Pairs = append(result.Pairs, pair)
		}
	}

	// Pass 2: fuzzy match unmatched resources by canonical name
	// Group unmatched left/right by canonical key (namespace + canonical name)
	type canonEntry struct {
		key      string
		resource model.Resource
	}
	leftCanon := make(map[string][]canonEntry)
	for key, r := range leftMap {
		if matchedLeft[key] {
			continue
		}
		ck := canonicalKey(r)
		leftCanon[ck] = append(leftCanon[ck], canonEntry{key, r})
	}
	rightCanon := make(map[string][]canonEntry)
	for key, r := range rightMap {
		if matchedRight[key] {
			continue
		}
		ck := canonicalKey(r)
		rightCanon[ck] = append(rightCanon[ck], canonEntry{key, r})
	}

	// Pair fuzzy matches
	for ck, leftEntries := range leftCanon {
		rightEntries := rightCanon[ck]
		i := 0
		for ; i < len(leftEntries) && i < len(rightEntries); i++ {
			l := leftEntries[i]
			r := rightEntries[i]
			displayKey := l.key + " ~ " + nameFromKey(r.key)
			pair := makePair(displayKey, &l.resource, &r.resource)
			result.Pairs = append(result.Pairs, pair)
		}
		// Remaining unmatched left
		for ; i < len(leftEntries); i++ {
			l := leftEntries[i]
			result.Pairs = append(result.Pairs, model.ResourcePair{
				Key: l.key, Left: &l.resource, Status: model.StatusOnlyInLeft,
			})
		}
		// Remaining unmatched right (if left had fewer)
		for ; i < len(rightEntries); i++ {
			r := rightEntries[i]
			result.Pairs = append(result.Pairs, model.ResourcePair{
				Key: r.key, Right: &r.resource, Status: model.StatusOnlyInRight,
			})
		}
		delete(rightCanon, ck)
	}

	// Remaining right-only (no fuzzy match at all)
	for _, rightEntries := range rightCanon {
		for _, r := range rightEntries {
			result.Pairs = append(result.Pairs, model.ResourcePair{
				Key: r.key, Right: &r.resource, Status: model.StatusOnlyInRight,
			})
		}
	}

	// Sort pairs for stable output
	sort.Slice(result.Pairs, func(i, j int) bool {
		return result.Pairs[i].Key < result.Pairs[j].Key
	})

	return result
}

func makePair(key string, left, right *model.Resource) model.ResourcePair {
	pair := model.ResourcePair{Key: key, Left: left, Right: right}
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
	return pair
}

// canonicalKey returns a matching key with hex hashes stripped.
func canonicalKey(r model.Resource) string {
	cn := canonicalName(r.Name)
	if r.Namespace == "" {
		return cn
	}
	return r.Namespace + "/" + cn
}

// nameFromKey extracts just the name part from a "namespace/name" key.
func nameFromKey(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}
