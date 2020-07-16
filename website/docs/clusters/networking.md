---
title: Networking
---

## Connecting to other VPCs

When your app needs to connect to external services such as databases, they might already be hosted in different VPCs. In this situation, you'll need to set up VPC peering with the other VPC. In this example, we will walk through connecting a Konstellation VPC to an existing VPC so it could access an RDS instance.

1. Find the respective VPC ids and note them down. You can find the VPC Konstellation created by searching for kube-kon in the [VPC console](https://us-west-2.console.aws.amazon.com/vpc/home). From now on, it'll be referenced as vpc-kon.

  Similarly, for the RDS database, it will indicate the VPC that it resides in. It'll be referenced as vpc-rds from now on. Also, note the security group that your database is running in. You'll need this in step 3.

1. Open [Peering Connections](https://us-west-2.console.aws.amazon.com/vpc/home?region=us-west-2#PeeringConnections:sort=desc:vpcPeeringConnectionId) and click on Create Peering Connection.

  In this dialog, select VPC (Requester) to be `vpc-kon`, and VPC (Accepter) to be `vpc-rds`. It's also helpful to give it a descriptive name

1. Enable security

## Testing network connectivity
