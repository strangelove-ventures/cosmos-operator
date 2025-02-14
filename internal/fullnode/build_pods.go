package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configChecksumAnnotation = "cosmos.strange.love/config-checksum"
)

// BuildPods creates the final state of pods given the crd.
func BuildPods(crd *cosmosv1.CosmosFullNode, cksums ConfigChecksums) ([]diff.Resource[*corev1.Pod], error) {
	var (
		builder = NewPodBuilder(crd)
		pods    []diff.Resource[*corev1.Pod]
	)
	candidates := podCandidates(crd)
	for i := crd.Spec.Ordinals.Start; i < crd.Spec.Ordinals.Start+crd.Spec.Replicas; i++ {
		pod, err := builder.WithOrdinal(i).Build()
		if err != nil {
			return nil, err
		}

		if pod == nil {
			continue
		}

		if _, shouldSnapshot := candidates[pod.Name]; shouldSnapshot {
			continue
		}

		pod.Annotations[configChecksumAnnotation] = cksums[client.ObjectKeyFromObject(pod)]
		pods = append(pods, diff.Adapt(pod, i))
	}

	for i := crd.Spec.Ordinals.Start; i < crd.Spec.Ordinals.Start+crd.Spec.Replicas; i++ {
		// Build any additional versioned pods for this ordinal
		for podIdx, additionalPodSpec := range crd.Spec.AdditionalVersionedPods {
			additionalPod, err := buildAdditionalPod(crd, i, additionalPodSpec)
			if err != nil {
				return nil, err
			}

			if additionalPod == nil {
				continue
			}

			// Use a unique identifier for the additional pod (combining ordinal with pod index)
			// This ensures stability in the diff algorithm
			podOrdinal := i*100 + int32(podIdx) + 1000 // Add offset to ensure uniqueness
			additionalPod.Annotations[configChecksumAnnotation] = cksums[client.ObjectKeyFromObject(additionalPod)]
			pods = append(pods, diff.Adapt(additionalPod, podOrdinal))
		}
	}
	return pods, nil
}

func updatePodVersionLabel(pod *corev1.Pod, img string) {
	pod.Labels[kube.VersionLabel] = kube.ParseImageVersion(img)
}

func setVersionedImages(pod *corev1.Pod, v *cosmosv1.ChainVersion) {
	setChainContainerImage(pod, v.Image)

	for name, image := range v.InitContainers {
		for i := range pod.Spec.InitContainers {
			if pod.Spec.InitContainers[i].Name == name {
				pod.Spec.InitContainers[i].Image = image
				break
			}
		}
	}

	for name, image := range v.Containers {
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == name {
				pod.Spec.Containers[i].Image = image
				break
			}
		}
	}
}

func setChainContainerImage(pod *corev1.Pod, image string) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == mainContainer {
			pod.Spec.Containers[i].Image = image
			break
		}
	}

	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == chainInitContainer {
			pod.Spec.InitContainers[i].Image = image
			break
		}
	}
}

func podCandidates(crd *cosmosv1.CosmosFullNode) map[string]struct{} {
	candidates := make(map[string]struct{})
	for _, v := range crd.Status.ScheduledSnapshotStatus {
		candidates[v.PodCandidate] = struct{}{}
	}
	return candidates
}
