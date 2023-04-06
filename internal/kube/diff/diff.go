package kube

import (
	"errors"
	"sort"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ordinalAnnotation = "app.kubernetes.io/ordinal"
	revisionLabel     = "app.kubernetes.io/revision"
)

// Resource is a diffable kubernetes object.
type Resource[T client.Object] interface {
	Object() T
	Revision() string
	// Ordinal returns the ordinal position of the resource. If order doesn't matter, return 0.
	Ordinal() int64
}

type ordinalSet[T client.Object] map[client.ObjectKey]Resource[T]

// Diff computes steps needed to bring a current state equal to a new state.
// Diffing for updates is done by comparing a revision label.
//
// Prefer using Diff over Diff. Diff uses semantic equality to detect updates instead of a dedicated label.
// Diff may eventually be deprecated.
//
// Diff expects "revisionLabelKey" which is a label with a revision that is expected to change if the resource
// has changed. A short hash is a common value for this label. We cannot simply diff the annotations and/or labels in case
// a 3rd party injects annotations or labels.
// For example, GKE injects other annotations beyond our control.
//
// There are several O(N) or O(2N) operations; However, we expect N to be small.
type Diff[T client.Object] struct {
	creates, deletes, updates []T
}

// TODO
//// NewOrdinalRevisionDiff creates a valid Diff where ordinal positioning is required.
//func NewOrdinalRevisionDiff[T Resource](ordinalAnnotationKey string, revisionLabelKey string, current, want []T) *Diff[T] {
//	return newDiff(ordinalAnnotationKey, revisionLabelKey, current, want, false)
//}

func New[T client.Object](current []T, want []Resource[T]) *Diff[T] {
	d := &Diff[T]{}

	currentSet := d.currentToSet(current)
	if len(currentSet) != len(current) {
		panic(errors.New("each resource in current must have unique .metadata.name"))
	}

	wantSet := d.toSet(want)
	if len(wantSet) != len(want) {
		panic(errors.New("each resource in want must have unique .metadata.name"))
	}

	d.creates = d.computeCreates(currentSet, wantSet)
	//d.deletes = d.computeDeletes(currentSet, wantSet)

	// updates must come last
	//d.updates = d.computeUpdates(currentSet, wantSet)
	return d
}

// Creates returns a list of resources that should be created from scratch.
func (diff *Diff[T]) Creates() []T {
	return diff.creates
}

func (diff *Diff[T]) computeCreates(current, want ordinalSet[T]) []T {
	var creates []Resource[T]
	for objKey, resource := range want {
		_, ok := current[objKey]
		if !ok {
			creates = append(creates, resource)
		}
	}
	return diff.toObjects(diff.sortByOrdinal(creates))
}

// Deletes returns a list of resources that should be deleted.
//func (diff *Diff[T]) Deletes() []T {
//	return diff.deletes
//}
//
//func (diff *Diff[T]) computeDeletes(current, want ordinalSet[T]) []T {
//	var deletes []Resource
//	for objKey, resource := range current {
//		_, ok := want[objKey]
//		if !ok {
//			deletes = append(deletes, resource)
//		}
//	}
//	return diff.sortByOrdinal(deletes)
//}

// Updates returns a list of resources that should be updated by comparing the revision label.
//func (diff *Diff[T]) Updates() []T {
//	return diff.updates
//}

// uses the revisionLabelKey to determine if a resource has changed thus requiring an update.
//func (diff *Diff[T]) computeUpdates(current, want ordinalSet[T]) []T {
//	var updates []ordinalResource[T]
//	for _, existing := range current {
//		target, ok := want[client.ObjectKeyFromObject(existing.Resource)]
//		if !ok {
//			continue
//		}
//		target.Resource.SetResourceVersion(existing.Resource.GetResourceVersion())
//		target.Resource.SetUID(existing.Resource.GetUID())
//		target.Resource.SetGeneration(existing.Resource.GetGeneration())
//		var (
//			oldRev = existing.Resource.Revision()
//			newRev = target.Resource.Revision()
//		)
//		if oldRev != newRev {
//			updates = append(updates, target)
//		}
//	}
//
//	return diff.sortByOrdinal(updates)
//}

type currentAdapter[T client.Object] struct {
	obj T
}

func (a currentAdapter[T]) Object() T        { return a.obj }
func (a currentAdapter[T]) Revision() string { return a.obj.GetLabels()[revisionLabel] }

func (a currentAdapter[T]) Ordinal() int64 {
	val, _ := strconv.ParseInt(a.obj.GetAnnotations()[ordinalAnnotation], 10, 64)
	return val
}

func (diff *Diff[T]) currentToSet(current []T) ordinalSet[T] {
	m := make(ordinalSet[T])
	for i := range current {
		r := current[i]
		m[client.ObjectKeyFromObject(r)] = currentAdapter[T]{r}
	}
	return m
}

func (diff *Diff[T]) toSet(list []Resource[T]) ordinalSet[T] {
	m := make(ordinalSet[T])
	for i := range list {
		r := list[i]
		m[client.ObjectKeyFromObject(r.Object())] = r
	}
	return m
}

func (diff *Diff[T]) sortByOrdinal(list []Resource[T]) []Resource[T] {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Ordinal() < list[j].Ordinal()
	})
	sorted := make([]Resource[T], len(list))
	for i := range list {
		sorted[i] = list[i]
	}
	return sorted
}

func (diff *Diff[T]) toObjects(list []Resource[T]) []T {
	objs := make([]T, len(list))
	for i := range list {
		obj := list[i].Object()

		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[revisionLabel] = list[i].Revision()
		obj.SetLabels(labels)

		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[ordinalAnnotation] = strconv.FormatInt(list[i].Ordinal(), 10)
		obj.SetAnnotations(annotations)

		objs[i] = obj
	}
	return objs
}
