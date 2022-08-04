package fullnode

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// PodHasChanges if lhs has changes from rhs.
// TODO: thought maybe produce a hash in the pod builder.
func PodHasChanges(lhs, rhs *corev1.Pod) bool {
	if len(lhs.Spec.Containers) != len(rhs.Spec.Containers) {
		return true
	}
	for i, lhsc := range lhs.Spec.Containers {
		rhsc := rhs.Spec.Containers[i]
		if lhsc.Name != rhsc.Name ||
			lhsc.Image != rhsc.Image ||
			lhsc.ImagePullPolicy != rhsc.ImagePullPolicy ||
			!cmp.Equal(lhsc.Ports, rhsc.Ports) ||
			!cmp.Equal(lhsc.Command, rhsc.Command) ||
			!cmp.Equal(lhsc.Args, rhsc.Args) ||
			!cmp.Equal(lhsc.Resources, rhsc.Resources, cmpopts.IgnoreUnexported(resource.Quantity{})) ||
			!cmp.Equal(lhsc.ReadinessProbe, rhsc.ReadinessProbe) {
			return true
		}
	}
	return lhs.Spec.TerminationGracePeriodSeconds != rhs.Spec.TerminationGracePeriodSeconds
}
