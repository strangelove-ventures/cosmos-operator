# Quick Start

This quick start guide creates a CosmosFullNode that runs as an RPC node for the Cosmos Hub.

### Prerequisites

You will need kuberentes nodes that can provide up to 32GB and 4CPU per replica. The following example
deploys 2 replicas.

### Install the CRDs and deploy operator in your cluster

View [docker images here](https://github.com/strangelove-ventures/cosmos-operator/pkgs/container/cosmos-operator).

```sh
# Deploy the latest release. Warning: May be a release candidate.
make deploy IMG="ghcr.io/strangelove-ventures/cosmos-operator:$(git describe --tags --abbrev=0)"

# Deploy a specific version
make deploy IMG="ghcr.io/strangelove-ventures/cosmos-operator:<version you choose>"
```

#### TODO

Helm chart coming soon.

### Choose a StorageClass

View storage classes in your cluster:
```sh
kubectl get storageclass
```

Choose one that provides SSD. On GKE, we recommend `premium-rwo`.

### Find latest mainnet version of Gaia

See "Recommended Version" on [Minstcan](https://www.mintscan.io/cosmos/info).

### Find seeds

Copy "Peers" -> "Seeds" on [Minstcan](https://www.mintscan.io/cosmos/info).

### Find a recent snapshot

We recommend [Polkachu](https://www.polkachu.com/tendermint_snapshots/cosmos). Copy the URL of the Download link.

### Create a CosmosFullNode

Using the information from the previous steps, create a yaml file using the below template.

Then `kubectl apply -f` the yaml file.

```yaml
apiVersion: cosmos.strange.love/v1
kind: CosmosFullNode
metadata:
  name: cosmoshub
  namespace: default
spec:
  chain:
    app:
      minGasPrice: 0.001uatom
    binary: gaiad
    chainID: cosmoshub-4
    config:
      seeds: <your seeds> # TODO
    genesisURL: https://snapshots.polkachu.com/genesis/cosmos/genesis.json
    network: mainnet
    skipInvariants: true
    snapshotURL: <your snapshot, probably from Polkachu> # TODO
  podTemplate:
    image: ghcr.io/strangelove-ventures/heighliner/gaia:<latest version of gaia> # TODO
    resources:
      requests:
        memory: 16Gi
  replicas: 2 # TODO change to 1 to use less resources
  volumeClaimTemplate:
    resources:
      requests:
        storage: 200Gi
    storageClassName: <your chosen storage class> # TODO
```

### Monitor pods

Once created, monitor the pods in the `default` namespace for any errors.