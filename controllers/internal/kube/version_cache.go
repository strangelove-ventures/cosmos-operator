package kube

import (
	"k8s.io/apimachinery/pkg/types"
)

// VersionCache keeps track of previous resource versions to detect resource changes.
//
// Kubernetes sets the resource version.
// https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions
//
// Currently, this cache grows unbounded. The cache is not goroutine-safe.
type VersionCache map[types.UID]string

// VersionedResource is a subset of client.Object.
type VersionedResource interface {
	// GetUID returns a resource unique id.
	// Every object created over the whole lifetime of a Kubernetes cluster has a distinct UID.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids
	GetUID() types.UID

	// GetResourceVersion is an opaque string set by kubernetes when a resource changes.
	GetResourceVersion() string
}

// NewVersionCache returns an initialized, non-nil VersionCache.
func NewVersionCache() VersionCache {
	return make(VersionCache)
}

// HasChanged returns true if the resource has changed from a previously cached resource version.
// Also returns true if the resource is not in the cache.
func (cache VersionCache) HasChanged(resource VersionedResource) bool {
	return cache[resource.GetUID()] != resource.GetResourceVersion()
}

// Update adds or replaces an entry with the version from resource.
func (cache VersionCache) Update(resource VersionedResource) {
	cache[resource.GetUID()] = resource.GetResourceVersion()
}
