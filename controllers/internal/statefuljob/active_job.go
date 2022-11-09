package statefuljob

import (
	"context"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
}

var isNotFoundErr = kube.IsNotFound

// FindActiveJob finds the currently active job in any state. A job is considered inactive if it cannot
// be found.
func FindActiveJob(ctx context.Context, getter Getter, crd *cosmosalpha.StatefulJob) (bool, *batchv1.Job, error) {
	job := new(batchv1.Job)
	job.Name = ResourceName(crd)
	job.Namespace = crd.Namespace
	err := getter.Get(ctx, client.ObjectKeyFromObject(job), job)
	switch {
	case isNotFoundErr(err):
		return false, nil, nil
	case err != nil:
		return false, nil, err
	}
	return true, job, nil
}
