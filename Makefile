test

# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/bharvest-devops/cosmos-operator:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

VERSION ?= v1
.PHONY: gen-api
gen-api: ## Generate new API resource. VERSION defaults to "v1". E.g. make gen-api KIND=CosmosNewResource VERSION=v1
ifndef KIND
	$(error KIND is not defined; e.g. KIND="CosmosMyNewResource")
endif
	@kubebuilder create api --group cosmos --kind $(KIND) --version $(VERSION)

CHAIN_NAME ?= $(error Please set CHAIN_NAME)
.PHONY: latest-snapshot
latest-snapshot: ## Get latest snapshot from polkachu. Must set CHAIN_NAME flag or env var.
	@curl -s https://polkachu.com/api/v1/chains/$(CHAIN_NAME)/snapshot | jq -r '.snapshot.url' | tr -d "\n"

.PHONY: test
test: manifests generate ## Run unit tests.
ifndef SKIP_TEST
	go test -race -short -cover -timeout=60s ./...
else
	echo "Warning: SKIP_TEST=$(SKIP_TEST). Skipping all tests!"
endif


.PHONY: tools
tools: ## Install dev tools.
	@# The below is the preferred way to install kubebuilder per https://book.kubebuilder.io/quick-start.html#installation
	@curl -s -L -o ./kubebuilder "https://go.kubebuilder.io/dl/latest/$$(go env GOOS)/$$(go env GOARCH)"
	@chmod +x ./kubebuilder && mv ./kubebuilder /usr/local/bin

	@go get -d sigs.k8s.io/kind
	@go mod tidy

##@ Build

.PHONY: build
build: generate ## Build manager binary.
	go build -o bin/manager .

.PHONY: run
run: manifests generate ## Run a controller from your host.
	go run . --log-level=debug

PRE_IMG ?= ghcr.io/bharvest-devops/cosmos-operator:dev$(shell git describe --always --dirty)
.PHONY: docker-prerelease
docker-prerelease: ## Build and push a prerelease docker image.
	#IMG=$(PRE_IMG) $(MAKE) docker-build docker-push
	IMG=$(PRE_IMG) $(MAKE) docker-build
	@echo "Pushed $(PRE_IMG)"

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
## If you run on MacOS, uncomment this under line
## docker buildx build -t ${IMG} --build-arg VERSION=$(shell echo ${IMG} | awk -F: '{print $$2}') --build-arg TARGETARCH="amd64" --build-arg BUILDARCH="arm64"  --platform=linux/amd64,linux/arm64 --push .
	docker build -t ${IMG} --build-arg VERSION=$(shell echo ${IMG} | awk -F: '{print $$2}') --build-arg TARGETARCH=amd64 --build-arg BUILDARCH=amd64 .

#.PHONY: docker-push
#docker-push: ## Push docker image with the manager.
#	docker push ${IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-prerelease
deploy-prerelease: install docker-prerelease ## Install CRDs, build docker image, and deploy a prerelease controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(PRE_IMG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -
	@#Hack to reset tag to avoid git thrashing.
	@cd config/manager && $(KUSTOMIZE) edit set image controller=ghcr.io/bharvest-devops/cosmos-operator:latest

#	@cd config/manager && $(KUSTOMIZE) edit set image controller=ghcr.io/strangelove-ventures/cosmos-operator:latest
	@cd config/manager && $(KUSTOMIZE) edit set image controller=ghcr.io/bharvest-devops/cosmos-operator:latest

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	#$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.9.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
