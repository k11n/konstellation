# Current Operator version
VERSION = $(shell sed -n 's/.*Version = "\(.*\)"/\1/p' < version/version.go)

# Image URL to use all building/pushing image targets
IMG ?= "k11n/operator:$(VERSION)"
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Generate deployable manifests to deploy/
deploy: manifests kustomize components
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/crd > deploy/crds.yaml
	$(KUSTOMIZE) build config/default > deploy/operator.yaml

# Installs operator onto the current cluster
install-operator:
	kubectl apply -f deploy/operator.yaml

uninstall-operator:
	kubectl delete -n kon-system deployment konstellation

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=konstellation webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif


# konstellation added rules
rice:
ifeq (, $(shell which rice))
	@{ \
	set -e ;\
	RICE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$RICE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get github.com/GeertJohan/go.rice/rice ;\
	rm -rf $$RICE_GEN_TMP_DIR ;\
	}
RICE=$(GOBIN)/rice
else
RICE=$(shell which rice)
endif


cli: rice
	@{ \
  		set -e ;\
  		find components/terraform -name ".terraform" -exec rm -r {} \; ;\
  		mkdir -p bin ;\
  		cd cmd/kon ;\
  		$(RICE) embed-go --import-path github.com/k11n/konstellation/cmd/kon/utils ;\
  		echo "Building cli" ;\
  		go build -i ;\
  		mv kon ../../bin/kon ;\
	}

prometheus-0.4:
	components/prometheus/build.py 0.4
	mv components/prometheus/0.4/dist/*.yaml deploy/kube-prometheus/0.4/

grafana: prometheus-0.4
	# build grafana-operator
	kustomize build components/grafana/operator > deploy/grafana/operator.yaml
	components/grafana/generate-resources.py components/prometheus/0.4/build/grafana-dashboardDefinitions.yaml
	kustomize build components/grafana > deploy/grafana/dashboards.yaml

components: prometheus-0.4 grafana

