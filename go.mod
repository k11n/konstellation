module github.com/k11n/konstellation

go 1.13

require (
	cloud.google.com/go v0.56.0 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.3 // indirect
	github.com/GeertJohan/go.rice v1.0.0
	github.com/apparentlymart/go-cidr v1.0.1
	github.com/aws/aws-sdk-go v1.31.9
	github.com/coreos/prometheus-operator v0.38.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/gammazero/workerpool v0.0.0-20200608033439-1a5ca90a5753
	github.com/go-logr/logr v0.1.0
	github.com/hako/durafmt v0.0.0-20200605151348-3a43fc422dd9
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.9
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/manifoldco/promptui v0.7.0
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.12.0 // indirect
	github.com/onsi/gomega v1.9.0 // indirect
	github.com/operator-framework/operator-sdk v0.17.1
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.0.11 // indirect
	github.com/spf13/cast v1.3.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/thoas/go-funk v0.6.0
	github.com/urfave/cli/v2 v2.2.0
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	golang.org/x/tools v0.0.0-20200403190813-44a64ad78b9b // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	gopkg.in/ini.v1 v1.55.0
	gopkg.in/yaml.v2 v2.3.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c
	istio.io/api v0.0.0-20200417223136-90a960729620
	istio.io/client-go v0.0.0-20200417172857-74af6f52f3d9
	istio.io/gogo-genproto v0.0.0-20200416215531-c23ae6ad14f9 // indirect
	k8s.io/api v0.18.3
	k8s.io/apiextensions-apiserver v0.18.2 // indirect
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-state-metrics v1.9.5 // indirect
	k8s.io/metrics v0.18.3
	k8s.io/utils v0.0.0-20200414100711-2df71ebbae66
	sigs.k8s.io/controller-runtime v0.6.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.38.1 // Required by operator-sdk 0.17.x
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
	k8s.io/utils => k8s.io/utils v0.0.0-20191114200735-6ca3b61696b6
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.5.2
)
