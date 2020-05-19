---
sidebarDepth: 2
---
# Getting Started

## Requirements

Konstellation works with EKS today. Google GKE and Azure AKS are both on the roadmap in the future. I welcome contributions from the community if that's what you are looking for. Right now we are focused on AWS.

Before you start, ensure that you have [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) and [terraform](https://learn.hashicorp.com/terraform/getting-started/install.html) installed.

## Installation

Konstellation CLI has been tested thoroughly MacOS.

### Mac

```bash
% brew tap k11n/konstellation
% brew install konstellation
```

When this is installed, run `kon --version` to confirm the CLI is correctly installed

## Configuration

```bash
% kon setup
Use the arrow keys to navigate: ↓ ↑ → ←
? Choose a cloud provider to configure:
  ▸ AWS
```
Konstellation requires a few pieces information before creating a cluster:

* AWS Access Key and Secret Key - used to manage AWS resources
* An S3 bucket to store Konstellation state
* Region(s) that you intend to use

This setup needs to be performed only once.

## Creating a cluster

```bash
% kon cluster create
```

This command walks you through the cluster creation process, there are a few decisions to make:

* [Network Topology](networking.md)
* Machine instance type
* Min & max size of the nodepool

For a test cluster, you can pick the defaults to get it quickly started. This will take a few minutes, Konstellation uses Terraform to configure the underlying VPC and the cluster.

If a Konstellation-compatible VPC is already available, it would be available as an option vs creating a new cluster. This is helpful when performing cluster migrations.

::: warning
This creates AWS resources that will incur costs that you will be responsible for.
:::

## Deploying your first app

First, create an app template with the CLI.

```bash
% kon app new
```

Enter your docker image and then open the generated yaml file and edit it. There are a few things you'll need to change:

* registry - if you are not using DockerHub, enter url of your docker registry
* ingress.hosts - one or more domains that you'd like your app to handle

Once complete, then load your config into Kubernetes with

```bash
% kon app load <yourapp>.yaml
```

That's it! With that, Konstellation will create all of the resources necessary to run your app on Kubernetes. It creates a native load balancer and outputs its address.

The app config is persisted in Kubernetes, and can be edited at any time with `kon app edit <app>`

### Checking app status

The status command gives an overview of the state of your app as currently deployed. It's useful to check up on different releases of the app, and load balancer status.

```
% kon app status <yourapp>

Target: production
Hosts: your.host.com
Load Balancer: b0d94b2f-istiosystem-konin-a4cf-358886547.us-west-2.elb.amazonaws.com
Scale: 1 min, 5 max

RELEASE                       BUILD              DATE                    PODS    STATUS    TRAFFIC
yourapp-20200423-1531-7800    yourrepo/image     2020-05-16 23:01:23     1/1     released  100%
```

### Routing your domain

What remains is creating a CNAME record that links your domain and the load balancer.

### Configuring SSL

## Cleaning it all up
