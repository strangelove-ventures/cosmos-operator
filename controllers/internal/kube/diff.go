package kube

import (
	"errors"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resource is a kubernetes resource.
type Resource interface {
	metav1.Object
}

// key is the resource name which must be unique per k8s conventions.
type ordinalSet[T Resource] map[string]ordinalResource[T]

func (set ordinalSet[T]) clone() ordinalSet[T] {
	c := make(ordinalSet[T])
	for k, v := range set {
		c[k] = v
	}
	return c
}

// OrdinalDiff computes steps needed to bring a current state equal to a new state.
type OrdinalDiff[T Resource] struct {
	ordinalAnnotationKey string
	hasChanges           HasChanges[T]

	creates, deletes, updates []T
}

// HasChanges returns true if want has changed from current.
type HasChanges[T Resource] func(current, want T) bool

// NewOrdinalDiff creates a valid OrdinalDiff.
// It computes differences between the "current" state needed to reconcile to the "want" state.
//
// OrdinalDiff expects resources with annotations denoting ordinal positioning similar to a StatefulSet. E.g. pod-0, pod-1, pod-2.
// The "ordinalAnnotationKey" is an annotation key whose value must be increasing integers. E.g. "0", "1", "2". The
// values must be unique within "current" and "want".
//
// There are several O(N) or O(2N) operations where N = number of resources.
// However, we expect N to be small.
func NewOrdinalDiff[T Resource](ordinalAnnotationKey string, current, want []T, hasChanges HasChanges[T]) *OrdinalDiff[T] {
	d := &OrdinalDiff[T]{
		ordinalAnnotationKey: ordinalAnnotationKey,
		hasChanges:           hasChanges,
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
func (diff *OrdinalDiff[T]) Creates() []T {
	return diff.creates
}

func (diff *OrdinalDiff[T]) computeCreates(current, want ordinalSet[T]) []T {
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
func (diff *OrdinalDiff[T]) Deletes() []T {
	return diff.deletes
}

func (diff *OrdinalDiff[T]) computeDeletes(current, want ordinalSet[T]) []T {
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
func (diff *OrdinalDiff[T]) Updates() []T {
	return diff.updates
}

// deletes must be calculated first before calling this method.
func (diff *OrdinalDiff[T]) computeUpdates(current, want ordinalSet[T]) []T {
	candidates := current.clone()
	// Remove deletes; don't need to update resources that will be gone.
	for _, deleted := range diff.Deletes() {
		if _, ok := current[deleted.GetName()]; ok {
			delete(candidates, deleted.GetName())
		}
	}

	var updates []ordinalResource[T]
	for _, oldResource := range current {
		newResource, ok := want[oldResource.Resource.GetName()]
		if !ok {
			continue
		}
		if diff.hasChanges(oldResource.Resource, newResource.Resource) {
			updates = append(updates, newResource)
		}
	}

	return diff.sortByOrdinal(updates)
}

type ordinalResource[T Resource] struct {
	Resource T
	Ordinal  int64
}

func (diff *OrdinalDiff[T]) toSet(list []T) ordinalSet[T] {
	m := make(map[string]ordinalResource[T])
	for i := range list {
		r := list[i]
		n := MustToInt(r.GetAnnotations()[OrdinalAnnotation])
		m[r.GetName()] = ordinalResource[T]{
			Resource: r,
			Ordinal:  n,
		}
	}
	return m
}

func (diff *OrdinalDiff[T]) sortByOrdinal(list []ordinalResource[T]) []T {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Ordinal < list[j].Ordinal
	})
	sorted := make([]T, len(list))
	for i := range list {
		sorted[i] = list[i].Resource
	}
	return sorted
}
