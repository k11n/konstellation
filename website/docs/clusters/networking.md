---
title: Networking
---

## Connecting to other VPCs

When your app needs to connect to external services such as databases, they may be hosted in different VPCs. In this situation, you'll need to set up [VPC peering](https://docs.aws.amazon.com/vpc/latest/peering/what-is-vpc-peering.html) with the other VPC. In this example, we will walk through connecting a Konstellation VPC to an existing VPC so it could access an RDS instance.

1. Find the respective VPC ids and note them down. You can find the VPC Konstellation created by searching for kube-kon in the [VPC console](https://us-west-2.console.aws.amazon.com/vpc/home). From now on, it'll be referenced as vpc-kon. Keep a note of the **vpc id** and **cidr block**.

  Similarly, for the RDS database, it will indicate the VPC that it resides in. It'll be referenced as vpc-rds from now on. Also, note the **security group** that your database is running in. You'll need this in step 3.

  ![RDS Info](/img/screen/rds-info.png)

1. Open [Peering Connections](https://us-west-2.console.aws.amazon.com/vpc/home?region=us-west-2#PeeringConnections:sort=desc:vpcPeeringConnectionId) and click on Create Peering Connection.

  ![Create Peering](/img/screen/create-peering.png)

  In this dialog, select VPC (Requester) to be `vpc-kon`, and VPC (Accepter) to be `vpc-rds`. It's also helpful to give it a descriptive name

1. Then select this new peering connection and accept the request from the Actions menu.

1. Enable inbound requests from vpc-kon. Find the security group that your database is hosted in (as notedin step 1). Click on "Edit Inbound Rules", and add a new rule allowing `All traffic` from the cidr block from step 1.

  ![Subnet Inbound Connections](/img/screen/subnet-inbound.png)

  Save rules and your VPCs should be connected.

## Testing network connectivity

It can be tricky to debug networking issues, especially without shell access from the cluster. Konstellation provides a utility that gives you shell access into the cluster.

Use `kon cluster shell` to get access to a new debugging pod created inside of the cluster. By default it'll pull the latest debian image. You can then install any packages that you might need for debugging purposes.

Using host, curl, or ping commands from within the cluster would let you test network connectivity to a destination address. For example, if a curl command is stuck, it's usually indicative of improperly configured peering or security group.
