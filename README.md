# cosmos-operator
Cosmos Operator manages custom resource definitions (CRDs) for full nodes (aka RPC nodes) and eventually validator nodes for blockchains created with the [Cosmos SDK](https://v1.cosmos.network/sdk).

## CosmosFullNode

The CosmosFullNode creates a highly available, fault-tolerant [full node](https://docs.cosmos.network/main/run-node/run-node.html) deployment.

The CosmosFullNode controller acts like a hybrid between a StatefulSet and a Deployment.
Like a StatefulSet, each pod has a corresponding persistent volume to manage blockchain state and data.
You can configure rolling updates similar to a Deployment.

Additionally, because full node persistent data can be destroyed and recreated with little consequence, the controller 
will destroy/recreate PVCs which is different from StatefulSets which never delete PVCs.
Deleting a CosmosFullNode also cleans up PVCs.

## Validators?

Coming soon!

## Getting Started

Run these commands to setup your environment:

```shell
make tools
```

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/cosmos-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/cosmos-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

# Best Practices

### Volumes, PVCs and StorageClass

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

## Using Volume Snapshots

TODO: How to use snapscheduler to create and restore from a kubernetes volume snapshot.

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
