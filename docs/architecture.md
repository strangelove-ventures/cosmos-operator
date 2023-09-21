# Cosmos Operator Architecture

This is a high-level overview of the architecture of the Cosmos Operator. It is intended to be a reference for
developers.

## Overview

The operator was written with the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework.

Kubebuilder simplifies and provides abstractions for creating a controller.

In a nutshell, an operator observes
a [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/). Its job is to match
cluster state with the desired state in the CRD. It
continually watches for changes and updates the cluster accordingly - a "control loop" pattern.

Each controller implements a Reconcile method:

```go
Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
```

Unlike "built-in" controllers like Deployments or StatefulSets, operators are visible in the cluster. It is one pod
backed by a Deployment under the cosmos-operator-system namespace.

A controller can watch resources outside of the CRD it manages. For example, CosmosFullNode watches for pod deletions,
so
it can spin up new pods if a user deletes one manually.

The watching of resources happens in this method for each controller:

```go
SetupWithManager(ctx context.Context, mgr ctrl.Manager) error
```

Refer to kubebuilder docs for more info.

### Makefile

Kubebuilder generated much of the Makefile. It contains common tasks for developers.

### `api` directory

This directory contains the different CRDs.

You should run `make generate manifests` each time you change CRDs.

### `config` directory

The config directory contains kustomize files. Strangelove uses these files to deploy the operator (instead of a helm
chart). A helm chart is still pending but presents challenges in keeping the kustomize and helm code in sync.

### `controllers` directory

The controllers directory contains every controller.

This directory is not unit tested. The code in controllers should act like `main()` functions where it's mostly wiring
up of dependencies from `internal`.

Kubebuilder includes an integration test suite which you can see in `controllers/suite_test.go`. So far we ignore it.
Integration or e2e tests within the context of the test suite will be challenging given the dependency on a
blockchain network, such as syncing from peers, downloading snapshot, etc. Instead, I recommend a monitored staging
environment
which has continuous delivery.

### `internal` directory

Almost all the business logic lives in `internal` and contains unit tests.

# CosmosFullNode

This is the flagship CRD of the Cosmos Operator and contains the most complexity.

### Builder, Diff, and Control Pattern

Each resource has its own builder and controller (referred as "control" in this context). For example,
see `pvc_builder.go` and `pvc_control.go` which only manages PVCs. All builders should have file suffix `_builder.go`
and all control objects `_control.go`.

The "control"
pattern was loosely inspired by Kubernetes source code.

On each reconcile loop:

1. (On process start) control is initialized with a Diff and a Builder.
2. The builder builds the desired resources from the CRD.
3. Control fetches a list of existing resources.
4. Control uses Diff to compute a diff of the existing to the desired.
5. Control makes changes based on what Diff reports.

The "control" tests are **integration tests** where we mock out the Kubernetes API, but not the Builder or Diff. The
tests run quickly (like unit tests) because we do not make any network calls.

The Diff object (`type Diff[T client.Object] struct`) took several iterations to get right. There is probably little
need to tweak it further.

The hardest problem with diffing is determining updates. Essentially, Diff looks for a `Revision() string` method on the
resource and sets a revision annotation. The revision is a simple fnv hash. It compares `Revision` to the existing annotation. 
If different, we know it's an update.

Builders return a `diff.Resource[T]` which Diff can use. Therefore, Control does not need to adapt resources.

The fnv hash is computed from a resource's JSON representation, which has proven to be stable.

### Special Note on Updating Status

There are several controllers that update a
CosmosFullNode's [status subresource](https://book-v1.book.kubebuilder.io/basics/status_subresource):

* CosmosFullNode
* ScheduledVolumeSnapshot
* SelfHealing

Each update to the status subresource triggers another reconcile loop. We found multiple controllers updating status
caused race conditions. Updates were not applied or applied incorrectly. 
Some controllers read the status to take action, so it's important to preserve the integrity of the status.

Therefore, you must use the special `SyncUpdate(...)` method from `fullnode.StatusClient`. It ensures updates are
performed serially per CosmosFullNode.

# Scheduled Volume Snapshot

Scheduled Volume Snapshot takes periodic backups.

To preserve data integrity, it will temporarily delete a pod, so it can capture a PVC snapshot without any process
writing to it.

It uses a finite state machine pattern in the main reconcile loop.

# StatefulJob

StatefulJob periodically runs a job on an interval (crontab not supported yet). The purpose is to run a job that
attaches to a PVC created from a VolumeSnapshot.

It's the least developed of the CRDs.
