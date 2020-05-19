module github.com/k11n/konstellation

go 1.13

require (
	cloud.google.com/go v0.56.0 // indirect
	github.com/Azure/go-autorest/autorest v0.10.0 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.3 // indirect
	github.com/GeertJohan/go.rice v1.0.0
	github.com/akavel/rsrc v0.9.0 // indirect
	github.com/apparentlymart/go-cidr v1.0.1
	github.com/aws/aws-sdk-go v1.30.9
	github.com/coreos/prometheus-operator v0.38.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/gophercloud/gophercloud v0.10.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.9
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/lytics/base62 v0.0.0-20180808010106-0ee4de5a5d6d
	github.com/manifoldco/promptui v0.7.0
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/nkovacs/streamquote v1.0.0 // indirect
	github.com/olekukonko/tablewriter v0.0.4
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.0.11 // indirect
	github.com/spf13/cast v1.3.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/thoas/go-funk v0.6.0
	github.com/urfave/cli/v2 v2.2.0
	github.com/valyala/fasttemplate v1.1.0 // indirect
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904 // indirect
	golang.org/x/sys v0.0.0-20200413165638-669c56c373c4 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	gopkg.in/ini.v1 v1.55.0
	gopkg.in/yaml.v2 v2.2.8
	istio.io/api v0.0.0-20200417223136-90a960729620
	istio.io/client-go v0.0.0-20200417172857-74af6f52f3d9
	istio.io/gogo-genproto v0.0.0-20200416215531-c23ae6ad14f9 // indirect
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-state-metrics v1.9.5 // indirect
	k8s.io/utils v0.0.0-20200414100711-2df71ebbae66 // indirect
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
	k8s.io/utils => k8s.io/utils v0.0.0-20191114200735-6ca3b61696b6
)
