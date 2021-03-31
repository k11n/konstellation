module github.com/k11n/konstellation

go 1.14

require (
	github.com/GeertJohan/go.rice v1.0.2
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/aws/aws-sdk-go v1.33.7
	github.com/coreos/prometheus-operator v0.40.0
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/gammazero/workerpool v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026
	github.com/imdario/mergo v0.3.10
	github.com/manifoldco/promptui v0.7.0
	github.com/mitchellh/hashstructure v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pkg/errors v0.9.1
	github.com/spf13/cast v1.3.0
	github.com/stretchr/testify v1.6.1
	github.com/thoas/go-funk v0.7.0
	github.com/urfave/cli/v2 v2.2.0
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/sys v0.0.0-20210326220804-49726bf1d181 // indirect
	gopkg.in/ini.v1 v1.57.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	honnef.co/go/tools v0.1.3 // indirect
	istio.io/api v0.0.0-20210318170531-e6e017e575c5
	istio.io/client-go v1.9.2
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/metrics v0.18.2
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.2
)

replace k8s.io/client-go => k8s.io/client-go v0.19.9
