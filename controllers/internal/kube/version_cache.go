package kube

// VersionCache keeps track of previous resource versions to detect resource changes.
//
// Key is the resource name; value is the version.
// Kubernetes sets the resource version.
// https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions
//
// Currently, this cache grows unbounded. The cache is not goroutine-safe.
type VersionCache map[string]string

// VersionedResource returns information about a kubernetes resource.
type VersionedResource interface {
	GetResourceVersion() string
	GetName() string
}

// NewVersionCache returns an initialized, non-nil VersionCache.
func NewVersionCache() VersionCache {
	return make(VersionCache)
}

// HasChanged returns true if the resource has changed from a previously cached resource version.
// Also returns true if the resource is not in the cache.
func (cache VersionCache) HasChanged(resource VersionedResource) bool {
	return cache[resource.GetName()] != resource.GetResourceVersion()
}

// Update adds or replaces an entry with the version from resource.
func (cache VersionCache) Update(resource VersionedResource) {
	cache[resource.GetName()] = resource.GetResourceVersion()
}
