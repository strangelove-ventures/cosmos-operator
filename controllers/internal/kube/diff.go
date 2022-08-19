package kube

import (
	"errors"
	"fmt"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resource is a kubernetes resource.
type Resource = client.Object

// key is the resource name which must be unique per k8s conventions.
type ordinalSet[T Resource] map[string]ordinalResource[T]

// OrdinalDiff computes steps needed to bring a current state equal to a new state.
type OrdinalDiff[T Resource] struct {
	ordinalAnnotationKey string
	revisionLabelKey     string

	creates, deletes, updates []T
}

// NewOrdinalDiff creates a valid OrdinalDiff.
// It computes differences between the "current" state needed to reconcile to the "want" state.
//
// OrdinalDiff expects resources with annotations denoting ordinal positioning similar to a StatefulSet. E.g. pod-0, pod-1, pod-2.
// The "ordinalAnnotationKey" allows OrdinalDiff to sort resources deterministically.
// Therefore, resources must have ordinalAnnotationKey set to an integer value such as "0", "1", "2"
// otherwise this function panics.
//
// OrdinalDiff also expects "revisionLabelKey" which is a label with a revision that is expected to change if the resource
// has changed. A short hash is a common value for this label. We cannot simply diff the annotations and/or labels in case
// a 3rd party injects annotations or labels.
// For example, GKE injects other annotations beyond our control.
//
// For Updates to work properly, OrdinalDiff uses ObjectHasChanges. Concretely, to detect updates the recommended path
// is changing annotations or labels.
//
// There are several O(N) or O(2N) operations where N = number of resources.
// However, we expect N to be small.
func NewOrdinalDiff[T Resource](ordinalAnnotationKey string, revisionLabelKey string, current, want []T) *OrdinalDiff[T] {
	d := &OrdinalDiff[T]{
		ordinalAnnotationKey: ordinalAnnotationKey,
		revisionLabelKey:     revisionLabelKey,
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

// Updates returns a list of resources that should be updated.
func (diff *OrdinalDiff[T]) Updates() []T {
	return diff.updates
}

// uses the revisionLabelKey to determine if a resource has changed thus requiring an update.
func (diff *OrdinalDiff[T]) computeUpdates(current, want ordinalSet[T]) []T {
	var updates []ordinalResource[T]
	for _, existing := range current {
		target, ok := want[existing.Resource.GetName()]
		if !ok {
			continue
		}
		var (
			oldRev = existing.Resource.GetLabels()[diff.revisionLabelKey]
			newRev = target.Resource.GetLabels()[diff.revisionLabelKey]
		)
		if newRev == "" {
			// If revision isn't found on new resources, indicates a serious error with the Operator.
			panic(fmt.Errorf("%s missing revision label %s", existing.Resource.GetName(), diff.revisionLabelKey))
		}

		if oldRev != newRev {
			updates = append(updates, target)
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
		n := MustToInt(r.GetAnnotations()[diff.ordinalAnnotationKey])
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
