---
sidebarDepth: 2
---
# Getting Started

## Why Konstellation

Kubernetes has become the de-facto standard for running workloads on machines. It's been adopted by tech companies big and small. It's also got a vibrant ecosystem, with many wonderful projects that are built on top of Kubernetes, solving [these](https://github.com/kubernetes/autoscaler) [important](https://istio.io/) [problems](https://github.com/kubernetes-sigs/aws-alb-ingress-controller).

However, the learning curve remains steep for developers. For many, using kubernetes means spending days learning about various components, and copying YAML definitions from Medium articles to make it all work. Even when it's set up, it remains a challenge to operate it: from things like rolling back a bad release, to figuring out how to update components to a new version.

I built Konstellation to lower that barrier of entry, giving you all the tools to manage apps on Kubernetes. Konstellation is designed to be as easy to use as Heroku, with a focus on reproducibility and operations.

Konstellation is currently in beta. As with most "beta" software, you should expect bugs to be there and be willing to [report them](https://github.com/k11n/konstellation/issues). I'll do my best at addressing incoming issues as quickly as possible.

## Quick Start

### Requirements

Konstellation works with EKS today. Google GKE and Azure AKS are both on the roadmap in the future. I welcome contributions from the community if that's what you are looking for. Right now we are focused on AWS.

Before you start, ensure that you have [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed.

### Installation

Konstellation CLI has been tested thoroughly MacOS.

#### Mac

```bash
% brew tap k11n/konstellation
% brew install konstellation
```

When this is installed, run `kon --version` to confirm the CLI is correctly installed

### Configuration

```
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

### Creating a cluster

### Deploying your first app

### Routing your domain

### Configuring SSL
