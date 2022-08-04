package kube

import (
	"errors"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resource is a kubernetes resource.
type Resource = metav1.Object

// key is the resource name which must be unique per k8s conventions.
type ordinalSet[T Resource] map[string]ordinalResource[T]

// Diff computes steps needed to bring a current state equal to a new state.
type Diff[T Resource] struct {
	ordinalAnnotationKey string
	hashChanges          HasChanges[T]

	creates, deletes, updates []T
}

// HasChanges returns true if lhs is different than rhs.
type HasChanges[T Resource] func(lhs, rhs T) bool

// NewDiff creates a valid Diff.
// It computes differences between the "current" state needed to reconcile to the "want" state.
//
// Diff expects resources with annotations denoting ordinal positioning similar to a StatefulSet. E.g. pod-0, pod-1, pod-2.
// The ordinalAnnotationKey allows Diff to sort resources deterministically.
// Therefore, resources must have ordinalAnnotationKey set to an integer value such as "0", "1", "2"
// otherwise this function panics.
//
// For Updates to work properly,
//
// There are several O(N) or O(2N) operations where N = number of resources.
// However, we expect N to be small.
func NewDiff[T Resource](ordinalAnnotationKey string, current, want []T, hasChanges HasChanges[T]) *Diff[T] {
	d := &Diff[T]{
		ordinalAnnotationKey: ordinalAnnotationKey,
		hashChanges:          hasChanges,
	}

	currentSet := d.toSet(current)
	if len(currentSet) != len(current) {
		panic(errors.New("each resource in current must have unique .metadata.name"))
	}

	wantSet := d.toSet(want)
	if len(wantSet) != len(want) {
		panic(errors.New("each resource in want must have unique .metadata.name"))
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
	for name, resource := range current {
		_, ok := want[name]
		if !ok {
			deletes = append(deletes, resource)
		}
	}
	return diff.sortByOrdinal(deletes)
}

// Updates returns a list of resources that should be updated or patched.
func (diff *Diff[T]) Updates() []T {
	return diff.updates
}

// creates and deletes must be calculated first.
func (diff *Diff[T]) computeUpdates(current, want ordinalSet[T]) []T {
	var updates []ordinalResource[T]
	for _, existing := range current {
		target, ok := want[existing.Resource.GetName()]
		if !ok {
			continue
		}
		if diff.hashChanges(existing.Resource, target.Resource) {
			updates = append(updates, target)
		}
	}

	return diff.sortByOrdinal(updates)
}

type ordinalResource[T Resource] struct {
	Resource T
	Ordinal  int64
}

func (diff *Diff[T]) toSet(list []T) ordinalSet[T] {
	m := make(map[string]ordinalResource[T])
	for i := range list {
		r := list[i]
		n := MustToInt(r.GetAnnotations()[diff.ordinalAnnotationKey])
		m[r.GetName()] = ordinalResource[T]{
			Resource: r,
			Ordinal:  n,
		}
	}
	return m
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
