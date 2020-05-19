version = 0.1.0

deps:
	go get github.com/GeertJohan/go.rice/rice

cli: deps
	./cmd/kon/build.sh

operator:
	operator-sdk build "k11n/operator:v$(version)"

release-operator: operator
	docker push "k11n/operator:v$(version)"

run-operator:
	OPERATOR_NAME=k11n-operator operator-sdk run --local --operator-flags="--zap-devel"

all: cli operator

.PHONEY: deps release-operator run-operator
