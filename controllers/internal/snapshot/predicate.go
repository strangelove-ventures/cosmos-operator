package snapshot

import (
	"context"

	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Deleter is a subset of client.Client.
type Deleter interface {
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
}

// DeletePVCPredicate deletes the paired PVC for a job.
func DeletePVCPredicate(ctx context.Context, deleter Deleter) predicate.Predicate {
	logger := log.FromContext(ctx)

	return &predicate.Funcs{
		DeleteFunc: func(ev event.DeleteEvent) bool {
			// Purposefully panic if we're not handling a batch job.
			job := ev.Object.(*batchv1.Job)

			var pvc corev1.PersistentVolumeClaim
			pvc.Namespace = job.Namespace
			pvc.Name = job.Name

			logger.Info("Deleting PVC", "resource", pvc.Name)
			if err := deleter.Delete(ctx, &pvc); kube.IgnoreNotFound(err) != nil {
				logger.Error(err, "Failed to delete PVC", "pvc", pvc.Name)
			}

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
