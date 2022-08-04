package kube

import (
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectHasChanges returns true if lhs is different from rhs.
func ObjectHasChanges(lhs, rhs client.Object) bool {
	return lhs.GetName() != rhs.GetName() ||
		lhs.GetNamespace() != rhs.GetNamespace() ||
		lhs.GetObjectKind() != rhs.GetObjectKind() ||
		!cmp.Equal(lhs.GetLabels(), rhs.GetLabels()) ||
		!cmp.Equal(lhs.GetAnnotations(), rhs.GetAnnotations())
}
