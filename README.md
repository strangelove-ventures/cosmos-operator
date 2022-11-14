# Cosmos Operator

[![Project Status: Initial Release](https://img.shields.io/badge/repo%20status-active-green.svg?style=flat-square)](https://www.repostatus.org/#active)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue?style=flat-square&logo=go)](https://pkg.go.dev/github.com/strangelove-ventures/cosmos-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/strangelove-ventures/cosmos-operator)](https://goreportcard.com/report/github.com/strangelove-ventures/cosmos-operator)
[![License: Apache-2.0](https://img.shields.io/github/license/strangelove-ventures/cosmos-operator.svg?style=flat-square)](https://github.com/strangelove-ventures/cosmos-operator/blob/main/LICENSE)
[![Version](https://img.shields.io/github/tag/strangelove-ventures/cosmos-operator.svg?style=flat-square)](https://github.com/cosmos/strangelove-ventures/cosmos-operator)

Cosmos Operator is a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) for blockchains built with the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk). 

The long-term vision of this operator is to allow you to "configure it and forget it". 

## Motivation

Kubernetes provides a foundation for creating highly-available, scalable, fault-tolerant applications. 
Additionally, Kubernetes provides well-known DevOps patterns and abstractions vs. 
traditional DevOps which often requires "re-inventing the wheel".

Furthermore, the Operator Pattern allows us to mix infrastructure with business logic, 
thus minimizing human intervention and human error.

# Disclaimers

* Only tested on GKE. Although kubernetes is portable, YMMV with AWS, Azure, or other kubernetes providers. Or it may not work at all.
* Requires a recent version of kubernetes: v1.23+.
* CosmosFullNode: The chain must be built from the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk).
* CosmosFullNode: The controller requires [heighliner](https://github.com/strangelove-ventures/heighliner) images. If you build your own image, you will need a shell `sh` and set the uid:gid to 1025:1025.
* CosmosFullNode: May not work with all chains built with the Cosmos SDK. (Some chains diverge from common conventions.)

# CRDs

## CosmosFullNode

Status: v1, stable

CosmosFullNode is the flagship CRD. Its purpose is to deploy highly-available, fault-tolerant RPC nodes. 
It will eventually support validator sentries, persistent peers, and seeds, but is currently only suitable for RPC.

The CosmosFullNode controller acts like a hybrid between a StatefulSet and a Deployment.
Like a StatefulSet, each pod has a corresponding persistent volume to manage blockchain data.
But, you can also configure rolling updates similar to a Deployment.

As of this writing, Strangelove has been running CosmosFullNode in production for many weeks.

View a [minimal example](./config/samples/cosmos_v1_cosmosfullnode.yaml) or a [full example](./config/samples/cosmos_v1_cosmosfullnode_full.yaml).

### Roadmap

Disclaimer: Strangelove has not committed to these enhancements. They represent ideas that may or may not come to fruition.

* Scheduled upgrades. Set a halt height and image version. The controller performs a rolling update with the new image version after the committed halt height.
* Quicker p2p discovery using private peers. 
* Advanced readiness probe behavior. (The tendermint rpc status endpoint is not always reliable.)
* Automatic rollout for PVC resizing. (Currently human intervention required to restart pods after PVC resized.)
* Automatic PVC resizing. The controller increases PVC size once storage reaches a configured threshold; e.g. 80% full. (This may be a different CRD.)
* Bootstrap config using the chain registry. Query the chain registry and set config based on the registry.
* Validate p2p such as peers, seeds, etc. and filter out non-responsive peers.
* HPA support.
* Automatic upgrades. Controller monitors governance and performs upgrade without any human intervention.
* Corrupt data recovery. Detect when a PVC may have corrupted data. Restore data from a recent VolumeSnapshot.
* Safe, automatic backups. Create periodic VolumeSnapshots of PVCs while minimizing chance of data corruption during snapshot creation.

### Why not a StatefulSet?

CosmosFullNode gives you more control over individual pod and pvc pairs vs. a StatefulSet. You can configure pod/pvc 
pairs differently using the `instanceOverrides` feature. For example, data may become corrupt and the human operator needs 
to tweak or restore data.

Additionally, each pod requires slightly different config (such as peer settings in config.toml). Therefore, a blanket 
template as found in StatefulSet will not suffice.

In summary, we wanted precise control over resources so using an existing controller such as StatefulSet, Deployment, 
ReplicaSet, etc. was not a good fit.

## StatefulJob

Status: v1alpha1, experimental only

Currently, StatefulJob is alpha and will likely have backwards breaking changes. At this time, **we do not recommend using it in production**.

The StatefulJob is a means to process persistent data from a recent [VolumeSnapshot](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) of a PVC created from a CosmosFullNode. 
It periodically creates a job and PVC using the most recent VolumeSnapshot as its data source. It mounts the PVC as a volume into all of the job's pods. 
It's similar to a CronJob but does not offer advanced scheduling via a crontab. 

Strangelove uses it to compress and upload snapshots of chain data.

View [an example](./config/samples/cosmos_v1alpha1_statefuljob.yaml).

# Install in Your Cluster

View [images here](https://github.com/strangelove-ventures/cosmos-operator/pkgs/container/cosmos-operator).

```sh

make deploy IMG="ghcr.io/strangelove-ventures/cosmos-operator:$(git describe --tags --abbrev=0)"
```

TODO: Helm chart

# Contributing

See the [contributing guide](./docs/contributing.md).

# Best Practices

See the [best practices guide for CosmosFullNode](./docs/fullnode_best_practices.md).

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
