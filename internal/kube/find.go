package kube

import (
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindOrDefaultCopy returns a deep copy of the object if it exists in the collection, otherwise it
// returns a deep copy of the comparator.
// Also defaults .metadata.labels and .metadata.annotations to an empty map if they are nil.
func FindOrDefaultCopy[T client.Object](existing []T, comparator T) T {
	found := lo.FindOrElse(existing, comparator, func(item T) bool {
		return item.GetName() == comparator.GetName() && item.GetNamespace() == comparator.GetNamespace()
	})
	found = found.DeepCopyObject().(T)
	if found.GetLabels() == nil {
		found.SetLabels(map[string]string{})
	}
	if found.GetAnnotations() == nil {
		found.SetAnnotations(map[string]string{})
	}
	return found
}
