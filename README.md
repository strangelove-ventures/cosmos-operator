# Cosmos Operator

[![Project Status: Initial Release](https://img.shields.io/badge/repo%20status-active-green.svg?style=flat-square)](https://www.repostatus.org/#active)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue?style=flat-square&logo=go)](https://pkg.go.dev/github.com/strangelove-ventures/cosmos-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/strangelove-ventures/cosmos-operator)](https://goreportcard.com/report/github.com/strangelove-ventures/cosmos-operator)
[![License: Apache-2.0](https://img.shields.io/github/license/strangelove-ventures/cosmos-operator.svg?style=flat-square)](https://github.com/strangelove-ventures/cosmos-operator/blob/main/LICENSE)
[![Version](https://img.shields.io/github/tag/strangelove-ventures/cosmos-operator.svg?style=flat-square)](https://github.com/cosmos/strangelove-ventures/cosmos-operator)

Cosmos Operator is a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) primarily for blockchains built with the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk). It also supports [Penumbra](https://github.com/penumbra-zone/penumbra) and other chains which use [CometBFT](https://github.com/cometbft/cometbft) for consensus. 

The long-term vision of this operator is to allow you to "configure it and forget it". 

## Motivation

Kubernetes provides a foundation for creating highly-available, scalable, fault-tolerant applications. 
Additionally, Kubernetes provides well-known DevOps patterns and abstractions vs. 
traditional DevOps which often requires "re-inventing the wheel".

Furthermore, the Operator Pattern allows us to mix infrastructure with business logic, 
thus minimizing human intervention and human error.

# Disclaimers

* Tested on Google's GKE and Bare-metal with Kubeadm. Although kubernetes is portable, we cannot guarantee or provide support for AWS, Azure, or other kubernetes providers.
* Requires a recent version of kubernetes: v1.23+.
* CosmosFullNode: The chain must be built from the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk).
* CosmosFullNode: Validator sentries require a remote signer such as [horcrux](https://github.com/strangelove-ventures/horcrux).
* CosmosFullNode: The controller requires [heighliner](https://github.com/strangelove-ventures/heighliner) images. If you build your own image, you will need a shell `sh` and set the uid:gid to 1025:1025. If running as a validator sentry, you need `sleep` as well.
* CosmosFullNode: May not work for all Cosmos chains. (Some chains diverge from common conventions.) Strangelove has yet to encounter a Cosmos chain that does not work with this operator.

# CosmosFullNode CRD

Status: v1, stable

CosmosFullNode is the flagship CRD. Its purpose is to deploy highly-available, fault-tolerant blockchain nodes. 

The CosmosFullNode controller is like a StatefulSet for running Cosmos SDK blockchains.

A CosmosFullNode can be configured to run as an RPC node, a validator sentry, or a seed node. All configurations can
be used as persistent peers.

As of this writing, Strangelove has been running CosmosFullNode in production for over a year.

## Samples

[Minimal example yaml](./config/samples/cosmos_v1_cosmosfullnode.yaml)

[Full example yaml](./config/samples/cosmos_v1_cosmosfullnode_full.yaml)

[Penumbra example yaml](./config/samples/cosmos_v1_cosmosfullnode_penumbra.yaml)

### Why not a StatefulSet?
Each pod requires different config, such as peer settings in config.toml and mounted node keys. Therefore, a blanket
template as found in StatefulSet did not suffice.

Additionally, CosmosFullNode gives you more control over individual pod and pvc pairs vs. a StatefulSet to help
the human operator debug and recover from situations such as a corrupted PVCs.

# Support CRDs

These CRDs are part of the operator and serve to support CosmosFullNodes.

* [ScheduledVolumeSnapshot](./docs/scheduled_volume_snapshot.md)
* [StatefulJob](./docs/stateful_job.md)

# Quick Start

See the [quick start guide](./docs/quick_start.md).

# Contributing

See the [contributing guide](./CONTRIBUTING.md).

# Best Practices

See the [best practices guide for CosmosFullNode](./docs/fullnode_best_practices.md).

# Roadmap

Disclaimer: Strangelove has not committed to these enhancements and cannot estimate when they will be completed.

- [x] Scheduled upgrades. Set the upgrade height and image version, optionally setting halt height. The controller performs a rolling update with the new image version after the committed height.
- [x] Support configuration suitable for validator sentries.
- [x] Reliable, persistent peer support.
- [x] Quicker p2p discovery using private peers.
- [ ] Advanced readiness probe behavior. (The CometBFT rpc status endpoint is not always reliable.)
- [x] Automatic rollout for PVC resizing. (Currently human intervention required to restart pods after PVC resized.) Requires ExpandInUsePersistentVolumes feature gate.
- [x] Automatic PVC resizing. The controller increases PVC size once storage reaches a configured threshold; e.g. 80% full.
- [ ] Bootstrap config using the chain registry. Query the chain registry and set config based on the registry.
- [ ] Validate p2p such as peers, seeds, etc. and filter out non-responsive peers.
- [ ] HPA support.
- [ ] Automatic upgrades. Controller monitors governance and performs upgrade without any human intervention.
- [ ] Corrupt data recovery. Detect when a PVC may have corrupted data. Restore data from a recent VolumeSnapshot.
- [x] Safe, automatic backups. Create periodic VolumeSnapshots of PVCs while minimizing chance of data corruption during snapshot creation.

# License

Copyright 2023 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
