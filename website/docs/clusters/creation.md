---
title: Cluster Creation
---

## Network topology

Konstellation operates within its own VPC due to the complexities in networking setup. When you create the VPC, there are two network topologies to choose from for the VPC.

Both topologies ensures your load balancers are accessible from the internet. The tradeoffs are between levels of security and cost.

### Public

With this configuration, there's a single public subnet (per availability zone), and an internet gateway (IGW) to allow bidirectional communication over the internet. This means that every EKS node (EC2 instances) are reachable via the internet. To fine tune security and connection settings, use security groups associated with the EKS cluster.

This is a simpler topology, and more cost efficient.

![Public topology](/img/public-topology.png)

### Public + Private

With this configuration, for each availability zone, there'll be a public subnet, a private subnet, and a NAT gateway. In addition, there will be an internet gateway that communicates only with the public subnet.

With this configuration, all of your EKS nodes will be allocated in the private subnet, where they aren't assigned a public IP. Any outgoing traffic from EKS nodes must be routed through the NAT gateways. This provides added security, but it'll incur additional costs especially for traffic flowing through NAT gateways.

![Public topology](/img/publicprivate-topology.png)

## Node autoscaling



## Provider quotas

## Manging users

## Managing multiple clusters
