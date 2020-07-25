---
title: Cluster Creation
---

## Network topology

Konstellation needs to operate in a VPC that it has created due to complexities in networking setup. You may place other AWS services into the same VPC once it's created.

There are two network topologies to choose from when creating the VPC. Both topologies ensures your load balancers are accessible from the internet. The tradeoffs are between levels of security and cost.

We recommend using a /16 CIDR block. Konstellation will use one bit to denote public vs private, and 3 bits for the availability zones, leaving 12 bits or 4000 available IP addresses for each subnet.

When you create multiple clusters, they can either be put into the same VPC, or different VPCs. We'd recommend using the same VPC to reduce the amount of overhead in administration.

### Public

With this configuration, there's a single public subnet (per availability zone), and an internet gateway (IGW) to allow bidirectional communication over the internet. This means that every EKS node (EC2 instances) are reachable via the internet. To fine tune security and connection settings, use security groups associated with the EKS cluster.

This is a simpler topology, and more cost efficient.

![Public topology](/img/public-topology.png)

### Public + Private

With this configuration, for each availability zone, there'll be a public subnet, a private subnet, and a NAT gateway. In addition, there will be an internet gateway that communicates only with the public subnet.

With this configuration, all of your EKS nodes will be allocated in the private subnet, where they aren't assigned a public IP. Any outgoing traffic from EKS nodes must be routed through the NAT gateways. This provides added security, but it'll incur additional costs especially for traffic flowing through NAT gateways.

![Public topology](/img/publicprivate-topology.png)

## Cluster autoscaling

When you create a cluster, you'll be asked to specify the min and max number of nodes to use for the cluster. Konstellation will initially allocate the minimum, and then use the included [cluster autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) to scale it up.

As new apps are deployed, or when existing [apps scale up](../apps/basics.mdx#scaling), the cluster will allocate new nodes just in time.

The autoscaler will also scale down excess capacity, moving workload from under-utilized nodes before shutting them down.

## Provider quotas

One of the common reasons for VPC or cluster creation to fail is due to hitting service quotas with AWS. If you are seeing a failure, be sure to check the [EC2 limits page](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-resource-limits.html) and request an increase for the respective resource. Unfortunately, it's not easy to tell from the console which resource is close to the limits. The error message can usually give a clue to what failed.

Creates and deletes are idempotent in Konstellation. After resource limits have been increased, you can re-run the previously failed command. It should resume from where it left off.
