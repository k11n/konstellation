---
title: Deploying Your First App
---

## Creating a cluster

A cluster is the hosting container for apps. It represents a Kubernetes cluster, and will make use of a configurable number of nodes.

```bash
% kon cluster create
```

This command walks you through the cluster creation process, there are a few decisions to make:

* [Network Topology](../clusters/creation.md#network-topology)
* Machine instance type
* Min & max size of the nodepool

For a test cluster, you can pick the defaults to get it quickly started. This will take a few minutes, Konstellation uses Terraform to configure the underlying VPC and the cluster.

If a Konstellation-compatible VPC is already available, it would be available as an option vs creating a new cluster. This is helpful when performing cluster migrations.

:::caution
This creates AWS resources that will incur costs that you will be responsible for.
:::

## Selecting an active cluster

Konstellation supports working with multiple clusters. You will need to choose an active one in order to interact with it, including managing apps on that cluster.

```text
% kon cluster select <yourcluster>
```

## Deploying your first app

First, create an app manifest with the CLI.

```text
% kon app new
```

Enter your docker image and then edit the generated template. There are a few things you'll need to change:

* image - to test, enter `alexwhen/docker-2048`
* registry - if you are not using DockerHub, enter url of your docker registry
* ingress.hosts - one or more domains that you'd like the app to handle. Enter a hostname where you could control its DNS.

Once complete, then load the manifest into Kubernetes with

```
% kon app load <yourapp>.yaml
```

That's it! With that, Konstellation will create all of the resources necessary to run the app on Kubernetes, including a native load balancer.

The app manifest is persisted in Kubernetes, and can be edited at any time with `kon app edit <app>`

### Checking app status

The status command gives an overview of the state of an app as currently deployed. It's useful to check up on different releases of the app, and load balancer status.

```
% kon app status <yourapp>

Target: production
Hosts: your.host.com
Load Balancer: b0d94b2f-istiosystem-konin-a4cf-358886547.us-west-2.elb.amazonaws.com
Scale: 1 min, 5 max

RELEASE                       BUILD                  DATE                    PODS    STATUS    TRAFFIC
yourapp-20200423-1531-7800    alexwhen/docker-2048   2020-05-16 23:01:23     1/1     released  100%
```

### Routing your domain

What remains is linking your domain to the load balancer. You'll need to create an ALIAS (preferred on Route53) or CNAME record, and in the field, specify the `Load Balancer` address shown in the status output.

### Setting up SSL (Optional)

If you have a SSL certificate for the configured domain, Konstellation can [set up the load balancer to handle SSL termination](../apps/basics.mdx#setting-up-ssl).

## Cleaning it all up

When Konstellation creates VPC and cluster resources, it keeps track of the state to make it possible to delete those resources at a later point.

### Deleting your cluster

Clusters are removed with the `cluster destroy` subcommand. Remove the cluster you've created with `kon cluster destroy --cluster <yourcluster>`.

This process could take up to 10-15 minutes. So just hang tight and grab a drink. :sunglasses:

### Destroy VPC & networking resources

If you no longer want the VPC, or want to change your network topology, it's simple to destroy it and start over.

First check the ID of the VPC:

```text
% kon vpc list

aws (us-west-2)
------------------------------------------------------------------------
  VPC                     CIDR BLOCK    KONSTELLATION   TOPOLOGY
------------------------------------------------------------------------
  vpc-00a63c9cb05d5320f   10.1.0.0/16   yes             public_private
------------------------------------------------------------------------
```

Then destroy it.

```text
% kon vpc destroy --vpc <yourvpc>
```
