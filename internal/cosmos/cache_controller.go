package cosmos

import (
	"context"
	"fmt"
	"sync"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type cache struct {
	sync.RWMutex
	m map[client.ObjectKey]*cacheItem
}

type cacheItem struct {
	coll   StatusCollection
	cancel context.CancelFunc
}

func newCache() *cache {
	return &cache{
		m: make(map[client.ObjectKey]*cacheItem),
	}
}

func (c *cache) Get(key client.ObjectKey) (StatusCollection, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.m[key]
	if !ok {
		return nil, false
	}
	return v.coll, ok
}

func (c *cache) Init(key client.ObjectKey, cancel context.CancelFunc) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = &cacheItem{
		coll:   make(StatusCollection, 0),
		cancel: cancel,
	}
}

func (c *cache) Update(key client.ObjectKey, value StatusCollection) {
	c.Lock()
	defer c.Unlock()
	v, ok := c.m[key]
	if !ok {
		return
	}
	v.coll = value
}

func (c *cache) Del(key client.ObjectKey) {
	c.Lock()
	defer c.Unlock()
	if v, ok := c.m[key]; ok {
		v.cancel()
	}
	delete(c.m, key)
}

func (c *cache) DelAll() {
	c.Lock()
	defer c.Unlock()
	for _, v := range c.m {
		v.cancel()
	}
	c.m = make(map[client.ObjectKey]*cacheItem)
}

type Collector interface {
	Collect(ctx context.Context, pods []corev1.Pod) StatusCollection
}

const CacheControllerName = "CosmosCache"

// CacheController periodically polls pods for their CometBFT status and caches the result.
// The cache is a controller so it can watch CosmosFullNode objects to warm or invalidate the cache.
type CacheController struct {
	cache     *cache
	client    client.Reader
	collector Collector
	eg        errgroup.Group
	interval  time.Duration
	recorder  record.EventRecorder
}

func NewCacheController(collector Collector, reader client.Reader, recorder record.EventRecorder) *CacheController {
	return &CacheController{
		cache:     newCache(),
		client:    reader,
		collector: collector,
		interval:  5 * time.Second,
		recorder:  recorder,
	}
}

// SetupWithManager watches CosmosFullNode objects and starts cache collecting.
func (c *CacheController) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
	// We do not index pods because we presume another controller is already doing so.
	// If we repeat it here, the manager returns an error.
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.CosmosFullNode{}).
		Complete(c)
}

// Close stops all cache collecting and waits for all goroutines to exit.
func (c *CacheController) Close() error {
	c.cache.DelAll()
	return c.eg.Wait()
}

var finishResult reconcile.Result

func (c *CacheController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	crd := new(cosmosv1.CosmosFullNode)
	if err := c.client.Get(ctx, req.NamespacedName, crd); err != nil {
		if kube.IsNotFound(err) {
			c.cache.Del(req.NamespacedName)
		}
		return finishResult, kube.IgnoreNotFound(err)
	}

	reporter := kube.NewEventReporter(log.FromContext(ctx), c.recorder, crd)

	// If not already cached, start collecting status from pods.
	if _, ok := c.cache.Get(req.NamespacedName); !ok {
		cctx, cancel := context.WithCancel(ctx)
		c.cache.Init(req.NamespacedName, cancel)
		c.eg.Go(func() error {
			defer cancel()
			c.collectFromPods(cctx, reporter, req.NamespacedName)
			return nil
		})
	}

	return finishResult, nil
}

// Invalidate removes the given pods status from the cache.
func (c *CacheController) Invalidate(controller client.ObjectKey, pods []string) {
	v, _ := c.cache.Get(controller)
	now := time.Now()
	for _, s := range v {
		for _, pod := range pods {
			if s.Pod.Name == pod {
				s.Status = CometStatus{}
				s.Err = fmt.Errorf("invalidated")
				s.TS = now
			}
		}
	}
	c.cache.Update(controller, v)
}

// Collect returns a StatusCollection for the given controller. Only returns cached CometStatus.
func (c *CacheController) Collect(ctx context.Context, controller client.ObjectKey) StatusCollection {
	pods, err := c.listPods(ctx, controller)
	if err != nil {
		return nil
	}
	v, _ := c.cache.Get(controller)
	IntersectPods(&v, pods)
	for i := range pods {
		UpsertPod(&v, &pods[i])
	}
	return v
}

// SyncedPods returns only the pods that are ready and in sync (i.e. caught up with chain tip).
func (c *CacheController) SyncedPods(ctx context.Context, controller client.ObjectKey) []*corev1.Pod {
	return kube.AvailablePods(c.Collect(ctx, controller).SyncedPods(), 5*time.Second, time.Now())
}

func (c *CacheController) listPods(ctx context.Context, controller client.ObjectKey) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := c.client.List(ctx, &pods,
		client.InNamespace(controller.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: controller.Name},
	); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (c *CacheController) collectFromPods(ctx context.Context, reporter kube.Reporter, controller client.ObjectKey) {
	defer c.cache.Del(controller)

	collect := func() {
		pods, err := c.listPods(ctx, controller)
		if err != nil {
			err = fmt.Errorf("%s: %w", controller, err)
			reporter.Error(err, "Failed to list pods")
			reporter.RecordError("ListPods", err)
			return
		}
		c.cache.Update(controller, c.collector.Collect(ctx, pods))
	}

	collect() // Collect once immediately.
	tick := time.NewTicker(c.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			collect()
		}
	}
}
