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

For a test cluster, you can pick the defaults to get it quickly started. This will take a few minutes, Konstellation uses Terraform to configure the underlying VPC and the cluster. You could run multiple clusters in the same VPC.

## Deploying your first app

### Routing your domain

### Configuring SSL

## Cleaning it all up
