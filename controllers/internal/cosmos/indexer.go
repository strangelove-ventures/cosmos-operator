package cosmos

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IndexOwner returns field values for indexing "child" resources that are "owned" by a controller
// (typically the CRD such as CosmosFullNode).
// Indexing is required for client.Client methods such as listing resources.
//
// It returns a field to index only if all are true:
// 1) resource is part of cosmosv1.GroupVersion.
// 2) resource is owned by a controller equal to "kind".
func IndexOwner[T client.Object](kind string) client.IndexerFunc {
	return func(object client.Object) []string {
		resource := object.(T)
		owner := metav1.GetControllerOf(resource)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != cosmosv1.GroupVersion.String() || owner.Kind != kind {
			return nil
		}
		return []string{owner.Name}
	}
}
