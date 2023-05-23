package cosmos

import (
	"context"
	"sync"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type cache struct {
	sync.RWMutex
	m map[client.ObjectKey]StatusCollection
}

func (c *cache) Get(key client.ObjectKey) (StatusCollection, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *cache) Set(key client.ObjectKey, value StatusCollection) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = value
}

type Collector interface {
	Collect(ctx context.Context, pods []corev1.Pod) StatusCollection
}

// CacheController periodically polls pods for their CometBFT status and caches the result.
// The cache is a controller so it can watch CosmosFullNode objects to invalidate and warm the cache.
type CacheController struct {
	cache     *cache
	client    client.Reader
	collector Collector
}

func NewCacheController(collector Collector, reader client.Reader) *CacheController {
	return &CacheController{
		cache:     new(cache),
		client:    reader,
		collector: collector,
	}
}

// SetupWithManager watches CosmosFullNode objects.
func (c *CacheController) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
	// We do not index pods because we presume another controller is already doing so.
	// If we repeat it here, the manager returns an error.
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.CosmosFullNode{}).
		Complete(c)
}

var finishResult reconcile.Result

func (c *CacheController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	crd := new(cosmosv1.CosmosFullNode)
	if err := c.client.Get(ctx, req.NamespacedName, crd); err != nil {
		// TODO: remove from cache
		return finishResult, client.IgnoreNotFound(err)
	}

	// Fetch and add from cache.

	return finishResult, nil
}

// Collect returns a StatusCollection for the given controller. For optimal performance, only returns cached results.
// If the cache is stale, returns an empty StatusCollection.
func (c *CacheController) Collect(ctx context.Context, controller client.ObjectKey) StatusCollection {
	return nil
}
