version = $(shell sed -n 's/.*Version = "\(.*\)"/\1/p' < version/version.go)

deps:
	go get github.com/GeertJohan/go.rice/rice

cli: deps
	./cmd/kon/build.sh
	go install ./cmd/kon

operator:
	operator-sdk build "k11n/operator:v$(version)"

release-operator: operator
	docker push "k11n/operator:v$(version)"
	sed -i.bak 's/\(k11n\/operator:v\).*/\1$(version)/g' deploy/operator.yaml && rm deploy/operator.yaml.bak

run-operator:
	OPERATOR_NAME=k11n-operator operator-sdk run --local --operator-flags="--zap-devel"

generate:
	operator-sdk generate k8s
	operator-sdk generate crds

prometheus-0.4:
	cd components/prometheus/0.4; ./build.sh
	mkdir -p deploy/kube-prometheus/0.4
	mv components/prometheus/0.4/*.yaml deploy/kube-prometheus/0.4/

prometheus-0.3:
	cd components/prometheus/0.3; ./build.sh
	mkdir -p deploy/kube-prometheus/0.3
	mv components/prometheus/0.3/*.yaml deploy/kube-prometheus/0.3/

grafana:
	# build grafana-operator
	kustomize build components/grafana/operator > deploy/grafana/operator.yaml
	components/grafana/generate-resources.py components/prometheus/0.4/manifests/grafana-dashboardDefinitions.yaml
	kustomize build components/grafana > deploy/grafana/dashboards.yaml

components: prometheus-0.3 prometheus-0.4 grafana

test:
	go test ./...

all: cli operator

.PHONEY: deps release-operator run-operator test
