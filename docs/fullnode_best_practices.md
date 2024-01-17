# CosmosFullNode Best Practices

## Resource Names

If you plan to have multiple network environments in the same cluster or namespace, append the network name and any other identifying information.

Example:
```yaml
apiVersion: cosmos.bharvest/v1
kind: CosmosFullNode
metadata:
  name: cosmoshub-mainnet-fullnode
spec:
  chain:
    network: mainnet # Should align with metadata.name above.
```

Like a StatefulSet, the Operator uses the .metadata.name of the CosmosFullNode to name resources it creates and manages.

## Volumes, PVCs and StorageClass

Generally, Volumes are bound to a single Availability Zone (AZ). Therefore, use or define a [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/)
which has `volumeBindingMode: WaitForFirstConsumer`. This way, kubernetes will not provision the volume until there is a pod ready to bind to it.

If you do not configure `volumeBindingMode` to wait, you risk the scheduler ignoring pod topology rules such as `Affinity`.
For example, in GKE, volumes will be provisioned [in random zones](https://cloud.google.com/kubernetes-engine/docs/concepts/persistent-volumes).

The Operator cannot define a StorageClass for you. Instead, you must configure the CRD with a pre-existing StorageClass.

Cloud providers generally provide default StorageClasses for you. Some of them set `volumeBindingMode: WaitForFirstConsumer` such as GKE's `premium-rwo`.
```shell
kubectl get storageclass
```

Additionally, Cosmos nodes require heavy disk IO. Therefore, choose a faster StorageClass such as GKE's `premium-rwo`.

## Resizing Volumes

The StorageClass must support resizing. Most cloud providers (like GKE) support it.

To resize, update resources in the CRD like so:
```yaml
resources:
  requests:
    storage: 100Gi # increase size here
```

You can only increase the storage (never decrease).

You must manually watch the PVC for a status of `FileSystemResizePending`. Then manually restart the pod associated with the PVC to complete resizing.

The above is a workaround; there is [future work](https://github.com/bharvest-devops/cosmos-operator/issues/37) planned to allow the Operator to handle this scenario for you.

## Updating Volumes

Most PVC fields are immutable (such as StorageClass), so once the Operator creates PVCs, immutable fields are not updated even if you change values in the CRD.

As mentioned in the above section, you can only update the storage size.

If you need to update an immutable field like the StorageClass, the workaround is to `kubectl apply` the CRD. Then manually delete PVCs and pods. The Operator will recreate them with the new configuration.

There is [future work](https://github.com/bharvest-devops/cosmos-operator/issues/38) planned for the Operator to handle this scenario for you.

## Pod Affinity

The Operator cannot assume your preferred topology. Therefore, set affinity appropriately to fit your use case.

E.g. To encourage the scheduler to spread pods across nodes:

```yaml
template:
  affinity:
      podAntiAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                  - key: app.kubernetes.io/name
                    operator: In
                    values:
                      - <name of crd>
              topologyKey: kubernetes.io/hostname
```
