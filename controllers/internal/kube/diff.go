package kube

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resource is a kubernetes resource.
type Resource = metav1.Object

type ordinalSet[T Resource] map[string]ordinalResource[T]

// Diff computes steps needed to bring a current state equal to a new state.
type Diff[T Resource] struct {
	ordinalLabel     string
	creates, deletes []T
}

// NewDiff creates a valid Diff.
// It computes differences between the "current" state needed to reconcile to the "want" state.
//
// The "ordinalLabel" is a well-known label common to all resources. It's value must be a string which can be
// converted to an integer. It's used for proper sorting.
// Each resource name and ordinalLabel value must be unique.
//
// There are several O(N) or O(2N) operations where N = number of resources.
// However, we expect N to be small.
func NewDiff[T Resource](ordinalLabel string, current, want []T) *Diff[T] {
	d := &Diff[T]{ordinalLabel: ordinalLabel}

	currentSet := d.toMap(current)
	if len(currentSet) != len(current) {
		panic(errors.New("each resource in current must have unique .metadata.name"))
	}

	wantSet := d.toMap(want)
	if len(wantSet) != len(want) {
		panic(errors.New("each resource in want must have unique .metadata.name"))
	}

	d.creates = d.computeCreates(currentSet, wantSet)
	d.deletes = d.computeDeletes(currentSet, wantSet)

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

// IsDirty returns true if some resources are in a transitioning or non-clean state. E.g. A pod that is not Ready.
// Callers should check this value and take action, such as re-queueing in a controller.
func (diff *Diff[T]) IsDirty() bool {
	return false
}

// Updates returns a list of resources that should be updated or patched.
func (diff *Diff[T]) Updates() []T {
	// TODO (nix - 7/25/22) To be implemented
	return nil
}

type ordinalResource[T Resource] struct {
	Resource T
	Ordinal  int64
}

func (diff *Diff[T]) toMap(list []T) map[string]ordinalResource[T] {
	m := make(map[string]ordinalResource[T])
	for i := range list {
		r := list[i]
		v := r.GetLabels()[diff.ordinalLabel]
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			panic(fmt.Errorf("invalid ordinal label %q=%q: %w", diff.ordinalLabel, v, err))
		}
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
