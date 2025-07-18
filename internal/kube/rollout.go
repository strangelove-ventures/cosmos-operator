package kube

import (
	"errors"

	"k8s.io/apimachinery/pkg/util/intstr"
)

var defaultMaxUnavail = intstr.FromString("25%")

// ComputeRollout returns the number of replicas allowed to be updated to keep within
// a max unavailable threshold. Example: If max unavailable is 5 with a desired replica count of 10,
// that means rollout cannot happen until 6 or more replicas are ready. The replicas must stay
// within the minimum threshold of 5 ready replicas.
//
// If "maxUnavail" is nil, defaults to 25% and string value must be a percentage.
// "desired" must be >= 1 and "ready" must be >= 0 or else this function panics.
func ComputeRollout(maxUnavail *intstr.IntOrString, desired, ready int) int {
	if desired < 0 {
		panic(errors.New("desired must be >= 0"))
	}
	if ready < 0 {
		panic(errors.New("ready must be >= 0"))
	}

	if maxUnavail == nil {
		maxUnavail = &defaultMaxUnavail
	}
	unavail, err := intstr.GetScaledValueFromIntOrPercent(maxUnavail, desired, false)
	if err != nil {
		panic(err)
	}
	// At least 1 resource is allowed to be unavailable.
	if unavail < 1 {
		unavail = 1
	}
	minAvail := desired - unavail
	if ready <= minAvail {
		return 0
	}

	target := min(unavail-(desired-ready), desired)
	return target
}
