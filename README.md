# Konstellation - Application platform on Kubernetes

[![License](http://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)

Konstellation is a full stack application platform for Kubernetes. It provides an integrated set of tools that simplifies deployment of apps on k8s.

Companies often build proprietary tools to manage deployments on k8s, or deal with the raw resource YAMLs that can be complex and error-prone.

Konstellation is designed to lower the barrier of entry that comes with Kubernetes. Initially it's tightly integrated to support k8s clusters on AWS, with support for other cloud providers on the roadmap.

## Features

* Heroku-like usability on your own Kubernetes
* Cluster creation & management powered by Terraform
* Istio service mesh
* Custom resources that eliminate redundant/boilerplate YAML
* Release management & rollbacks
* Prometheus set up to scrape key app metrics
* Pre-configured Grafana dashboards for apps

For more see: [konstellation.dev](https://konstellation.dev)

[Documentation](https://konstellation.dev/docs/konstellation/introduction)

## Project Status

- [x] Alpha
- [x] Limited beta - initial users, possible schema changes
- [ ] Public beta - supports some production workloads
- [ ] General availability

## Installation

[Full Installation Docs](https://konstellation.dev/docs/)

Konstellation requires kubectl and terraform

### Mac

```
% brew tap k11n/konstellation
% brew install konstellation
```

### Build from source

```
% git clone https://github.com/k11n/konstellation.git
% cd konstellation
% make cli
% cp -Rv bin/kon /usr/local/bin
```

## Getting Started

[Getting Started Guide](https://konstellation.dev/docs/getting-started/deploy)

**Create a new cluster**

```
% kon cluster create
```

**Switch between clusters**

```
% kon cluster select <cluster>
```

**Deploy an app**

```
% kon app load <app.yaml>
...
% kon app status <app>
  Target:         production
  Ports:          http-80
  Hosts:          2048.mydomain.com
  Load balancer:  8846d32c-istiosystem-konin-a4cf-650024568.us-west-2.elb.amazonaws.com
  Scale:          1 min, 1 max

--------------------------------------------------------------------------------------------------------
  RELEASE                       BUILD                  DATE                  PODS   STATUS     TRAFFIC
--------------------------------------------------------------------------------------------------------
  app2048-20200806-0606-5cb08   alexwhen/docker-2048   2020-08-07 23:41:28   1/1    released   100%
--------------------------------------------------------------------------------------------------------
```

**Tail app logs**

```
% kon app logs -f <app>
```

**Shell into app pod**

```
% kon app shell <app>
```

## License

[Apache 2.0](LICENSE.txt)
