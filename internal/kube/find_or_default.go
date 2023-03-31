package kube

import (
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindOrDefault returns the object if it exists in the collection, otherwise it returns the comparator.
func FindOrDefault[T client.Object](existing []T, comparator T) T {
	return lo.FindOrElse(existing, comparator, func(item T) bool {
		return item.GetName() == comparator.GetName() && item.GetNamespace() == comparator.GetNamespace()
	})
}
