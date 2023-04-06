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
	// Revision returns a unique identifier for the resource. A simple hash of the resource is sufficient.
	Revision() string
	// Ordinal returns the ordinal position of the resource. If order doesn't matter, return 0.
	Ordinal() int64
}

type ordinalSet[T client.Object] map[client.ObjectKey]Resource[T]

// Diff computes steps needed to bring a current state equal to a new state.
// Diff will add annotations to created and updated resources required for future diffing.
//
// There are several O(N) or O(2N) operations; However, we expect N to be small.
type Diff[T client.Object] struct {
	creates, deletes, updates []T
}

// New creates a valid Diff.
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
	d.deletes = d.computeDeletes(currentSet, wantSet)
	d.updates = d.computeUpdates(currentSet, wantSet)
	return d
}

// Creates returns a list of resources that should be created from scratch.
// Calls are memoized, so you can call this method multiple times without incurring additional cost.
// Adds labels and annotations on the resource to aid in future diffing.
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
// Calls are memoized, so you can call this method multiple times without incurring additional cost.
func (diff *Diff[T]) Deletes() []T {
	return diff.deletes
}

func (diff *Diff[T]) computeDeletes(current, want ordinalSet[T]) []T {
	var deletes []Resource[T]
	for objKey, resource := range current {
		_, ok := want[objKey]
		if !ok {
			deletes = append(deletes, resource)
		}
	}
	return diff.toObjects(diff.sortByOrdinal(deletes))
}

// Updates returns a list of resources that should be updated.
// Calls are memoized, so you can call this method multiple times without incurring additional cost.
func (diff *Diff[T]) Updates() []T {
	return diff.updates
}

func (diff *Diff[T]) computeUpdates(current, want ordinalSet[T]) []T {
	var updates []Resource[T]
	for _, existing := range current {
		target, ok := want[client.ObjectKeyFromObject(existing.Object())]
		if !ok {
			continue
		}
		// These values are necessary to be accepted by the API server.
		target.Object().SetResourceVersion(existing.Object().GetResourceVersion())
		target.Object().SetUID(existing.Object().GetUID())
		target.Object().SetGeneration(existing.Object().GetGeneration())
		var (
			oldRev = existing.Revision()
			newRev = target.Revision()
		)
		if oldRev != newRev {
			updates = append(updates, target)
		}
	}

	return diff.toObjects(diff.sortByOrdinal(updates))
}

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
