package cosmos

import (
	"fmt"
	"sort"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resource is a kubernetes resource.
type Resource = metav1.Object

type Diff[T Resource] struct {
	ordinalLabel  string
	current, want map[string]ordinalResource[T]
}

//
// There is notably O(N) or O(2N) operations. However, we expect N (number of resources) to be small.
func NewDiff[T Resource](ordinalLabel string, current, want []T) *Diff[T] {
	d := &Diff[T]{ordinalLabel: ordinalLabel}
	// TODO: panic if lengths don't match
	d.current = d.toMap(current)
	d.want = d.toMap(want)
	return d
}

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
