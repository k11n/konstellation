module github.com/k11n/konstellation

go 1.13

require (
	github.com/GeertJohan/go.rice v1.0.0
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/aws/aws-sdk-go v1.33.7
	github.com/coreos/prometheus-operator v0.40.0
	github.com/gammazero/workerpool v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026
	github.com/imdario/mergo v0.3.10
	github.com/manifoldco/promptui v0.7.0
	github.com/mitchellh/hashstructure v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pkg/errors v0.9.1
	github.com/spf13/cast v1.3.0
	github.com/stretchr/testify v1.5.1
	github.com/thoas/go-funk v0.7.0
	github.com/urfave/cli/v2 v2.2.0
	gopkg.in/ini.v1 v1.57.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	istio.io/api v0.0.0-20200717202705-c1183dac172d
	istio.io/client-go v0.0.0-20200717004237-1af75184beba
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/metrics v0.18.2
	k8s.io/utils v0.0.0-20200720150651-0bdb4ca86cbc
	sigs.k8s.io/controller-runtime v0.6.0
)

replace k8s.io/client-go => k8s.io/client-go v0.18.2
