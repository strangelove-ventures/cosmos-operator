package cosmos

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resource is a kubernetes resource.
type Resource = metav1.Object

// Diff computes steps needed to bring a current state equal to a new state.
//
// With some methods there may be several O(N) or O(2N) operations where N = number of resources.
// However, we expect N to be small.
type Diff[T Resource] struct {
	ordinalLabel  string
	current, want map[string]ordinalResource[T]
}

// NewDiff creates a valid Diff.
// It computes differences between the "current" state needed to reconcile to the "want" state.
//
// The "ordinalLabel" is a well-known label common to all resources. It's value must be a string which can be
// converted to an integer.
// Each resource name and ordinalLabel value must be unique.
func NewDiff[T Resource](ordinalLabel string, current, want []T) *Diff[T] {
	d := &Diff[T]{ordinalLabel: ordinalLabel}

	d.current = d.toMap(current)
	if len(d.current) != len(current) {
		panic(errors.New("each resource in current must have unique .metadata.name"))
	}

	d.want = d.toMap(want)
	if len(d.want) != len(want) {
		panic(errors.New("each resource in want must have unique .metadata.name"))
	}

	return d
}

// Creates returns a list of resources that should be created anew.
func (diff *Diff[T]) Creates() []T {
	var creates []ordinalResource[T]
	for name, resource := range diff.want {
		_, ok := diff.current[name]
		if !ok {
			creates = append(creates, resource)
		}
	}
	return diff.sortByOrdinal(creates)
}

// Deletes returns a list of resources that should be deleted.
func (diff *Diff[T]) Deletes() []T {
	var deletes []ordinalResource[T]
	for name, resource := range diff.current {
		_, ok := diff.want[name]
		if !ok {
			deletes = append(deletes, resource)
		}
	}
	return diff.sortByOrdinal(deletes)
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
