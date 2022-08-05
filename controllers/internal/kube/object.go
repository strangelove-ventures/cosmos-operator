package kube

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectHasChanges returns true if lhs is different from rhs.
func ObjectHasChanges(lhs, rhs metav1.Object) bool {
	equal := lhs.GetName() == rhs.GetName() &&
		lhs.GetNamespace() == rhs.GetNamespace() &&
		reflect.DeepEqual(lhs.GetLabels(), rhs.GetLabels()) &&
		reflect.DeepEqual(lhs.GetAnnotations(), rhs.GetAnnotations())
	return !equal
}
