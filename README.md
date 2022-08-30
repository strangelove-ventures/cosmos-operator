# cosmos-operator
Cosmos Operator manages custom resource definitions (CRDs) for full nodes (aka RPC nodes) and eventually validator nodes for blockchains created with the [Cosmos SDK](https://v1.cosmos.network/sdk).

The long-term vision of the Operator is to allow you to "configure it and forget it". 

## CosmosFullNode

The CosmosFullNode creates a highly available, fault-tolerant [full node](https://docs.cosmos.network/main/run-node/run-node.html) deployment.

The CosmosFullNode controller acts like a hybrid between a StatefulSet and a Deployment.
Like a StatefulSet, each pod has a corresponding persistent volume to manage blockchain state and data.
But, you can also configure rolling updates similar to a Deployment.

Additionally, because full node persistent data can be destroyed and recreated with little consequence, the controller 
will clean up PVCs which is different from StatefulSets which never delete PVCs. Deleting a CosmosFullNode also cleans up PVCs.

## Validators?

Coming soon!

# Best Practices

## Resource Names

If you plan to have multiple network environments in the same cluster or namespace, append the network name and any other identifying information.

Example:
```yaml
apiVersion: cosmos.strange.love/v1
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

The above is a workaround; there is [future work](https://github.com/strangelove-ventures/cosmos-operator/issues/37) planned to allow the Operator to handle this scenario for you.

## Updating Volumes

Most PVC fields are immutable (such as StorageClass), so once the Operator creates PVCs, immutable fields are not updated even if you change values in the CRD.

As mentioned in the above section, you can only update the storage size.

If you need to update an immutable field like the StorageClass, the workaround is to `kubectl apply` the CRD. Then manually delete PVCs and pods. The Operator will recreate them with the new configuration.

There is [future work](https://github.com/strangelove-ventures/cosmos-operator/issues/38) planned for the Operator to handle this scenario for you.

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

## Using Volume Snapshots

TODO: How to use snapscheduler to create and restore from a kubernetes volume snapshot.

# Getting Started

Run these commands to setup your environment:

```shell
make tools
```

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

## Running a Prerelease on the Cluster

1. Authenticate with docker to push images to repository.

Create a [PAT on Github](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) with package read and write permissions.

```sh
printenv GH_PAT | docker login ghcr.io -u <your GH username> --password-stdin 
```

2. If a new cluster, install image pull secret.

*If project is now open source, omit and delete this step!*

```sh
GH_USER=<your Github username> GH_PAT=<personal access token> make regred
```

3. Deploy a prerelease.

*Warning: Make sure you're kube context is set appropriately, so you don't install in the wrong cluster!*

```sh
make deploy-prerelease
```

## Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

## Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

# Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

## How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster

## Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

## Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

# License

Copyright 2022 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
