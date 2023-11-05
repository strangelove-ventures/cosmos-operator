# Tests

We strive to 80% or higher unit test coverage. If your code is not well tested, your PR will not be merged.

Run tests via:
```sh
make test
```

# Architecture

For a high-level overview of the architecture, see [docs/architecture.md](./docs/architecture.md).

# Release Process

Prereq: Write access to the repo.

Releases should follow https://0ver.org.

1. Create and push a git tag on branch `main`. `git tag v0.X.X && git push --tags`
2. Triggers CICD action to build and push docker image to ghcr.
3. When complete, view the docker image in packages.

# Local Development 

The [Makefile](../Makefile) is your friend. Run `make help` to see all available targets.

Run these commands to setup your environment:

```shell
make tools
```

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

## Running a Prerelease on the Cluster

Prereq: Write access to the repo.

1. Authenticate with docker to push images to repository.

Create a [PAT on Github](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) with package read and write permissions.

```sh
printenv GH_PAT | docker login ghcr.io -u <your GH username> --password-stdin 
```

2. Deploy a prerelease.

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

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
