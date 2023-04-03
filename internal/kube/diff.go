package kube

import (
	"errors"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// key is the resource name which must be unique per k8s conventions.
type ordinalSet[T Resource] map[string]ordinalResource[T]

// Resource is a kubernetes resource.
type Resource = client.Object

// Diff computes a diff between two sets of resources.
// The diff is computed by comparing the semantic equality of the resources.
// This assumes building resources copies and modifies existing resources from kube, particularly regarding annotations.
// Cloud providers commonly add custom annotations to resources.
//
// There are several O(N) or O(2N) operations; however, we expect N to be small.
type Diff[T Resource] struct {
	ordinalAnnotationKey string
	isOrdinal            bool

	creates, deletes, updates []T
}

// NewOrdinalDiff creates a valid Diff where ordinal positioning is required.
func NewOrdinalDiff[T Resource](ordinalAnnotationKey string, current, want []T) *Diff[T] {
	return newDiff(ordinalAnnotationKey, current, want, true)
}

// NewDiff creates a valid Diff where ordinal positioning is not required.
func NewDiff[T Resource](current, want []T) *Diff[T] {
	return newDiff("", current, want, false)
}

func newDiff[T Resource](ordinalAnnotationKey string, current, want []T, isOrdinal bool) *Diff[T] {
	d := &Diff[T]{
		ordinalAnnotationKey: ordinalAnnotationKey,
		isOrdinal:            isOrdinal,
	}

	currentSet := d.toSet(current)
	if len(currentSet) != len(current) {
		panic(errors.New("each resource in current must have unique .metadata.name and namespace combination"))
	}

	wantSet := d.toSet(want)
	if len(wantSet) != len(want) {
		panic(errors.New("each resource in want must have unique .metadata.name and namespace combination"))
	}

	d.creates = d.computeCreates(currentSet, wantSet)
	d.deletes = d.computeDeletes(currentSet, wantSet)

	// updates must come last
	d.updates = d.computeUpdates(currentSet, wantSet)
	return d
}

// Creates returns a list of resources that should be created from scratch.
func (diff *Diff[T]) Creates() []T {
	return diff.creates
}

func (diff *Diff[T]) computeCreates(current, want ordinalSet[T]) []T {
	var creates []ordinalResource[T]
	for name, resource := range want {
		_, ok := current[name]
		if !ok {
			creates = append(creates, resource)
		}
	}
	return diff.sortByOrdinal(creates)
}

// Deletes returns a list of resources that should be deleted.
func (diff *Diff[T]) Deletes() []T {
	return diff.deletes
}

func (diff *Diff[T]) computeDeletes(current, want ordinalSet[T]) []T {
	var deletes []ordinalResource[T]
	for objID, resource := range current {
		_, ok := want[objID]
		if !ok {
			deletes = append(deletes, resource)
		}
	}
	return diff.sortByOrdinal(deletes)
}

// Updates returns a list of resources that should be updated.
func (diff *Diff[T]) Updates() []T {
	return diff.updates
}

// uses the revisionLabelKey to determine if a resource has changed thus requiring an update.
func (diff *Diff[T]) computeUpdates(current, want ordinalSet[T]) []T {
	var updates []ordinalResource[T]
	for _, existing := range current {
		target, ok := want[diff.objectKey(existing.Resource)]
		if !ok {
			continue
		}
		if !equality.Semantic.DeepEqual(existing.Resource, target.Resource) {
			updates = append(updates, target)
		}
	}

	return diff.sortByOrdinal(updates)
}

func (diff *Diff[T]) toSet(list []T) ordinalSet[T] {
	m := make(map[string]ordinalResource[T])
	for i := range list {
		r := list[i]
		var n int64
		if !diff.isOrdinal {
			n = MustToInt(r.GetAnnotations()[diff.ordinalAnnotationKey])
		}
		m[diff.objectKey(r)] = ordinalResource[T]{
			Resource: r,
			Ordinal:  n,
		}
	}
	return m
}

func (diff *Diff[T]) objectKey(obj client.Object) string {
	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}

func (diff *Diff[T]) sortByOrdinal(list []ordinalResource[T]) []T {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Ordinal < list[j].Ordinal
	})
	sorted := make([]T, len(list))
	for i := range list {
		sorted[i] = list[i].Resource
	}
	return sorted
}
