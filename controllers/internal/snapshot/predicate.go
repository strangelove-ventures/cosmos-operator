package snapshot

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DeletePredicate watches all delete events.
func DeletePredicate() predicate.Predicate {
	return &predicate.Funcs{
		DeleteFunc: func(ev event.DeleteEvent) bool {
			return true
		},
	}
}

// LabelSelectorPredicate returns a predicate matching default labels created by the operator.
func LabelSelectorPredicate() predicate.Predicate {
	pred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: defaultLabels()})
	if err != nil {
		panic(err)
	}
	return pred
}
