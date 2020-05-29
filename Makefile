version = $(shell sed -n 's/.*Version = "\(.*\)"/\1/p' < version/version.go)

deps:
	go get github.com/GeertJohan/go.rice/rice

cli: deps
	./cmd/kon/build.sh

operator:
	operator-sdk build "k11n/operator:v$(version)"

release-operator: operator
	docker push "k11n/operator:v$(version)"
	sed -i.bak 's/\(k11n\/operator:v\).*/\1$(version)/g' deploy/operator.yaml && rm deploy/operator.yaml.bak

run-operator:
	OPERATOR_NAME=k11n-operator operator-sdk run --local --operator-flags="--zap-devel"

all: cli operator

.PHONEY: deps release-operator run-operator vs
