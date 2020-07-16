---
title: Managing Users
---

When an EKS cluster is setup, by default it grants permissions to only the IAM user that created the cluster. Even when other users have the appropriate IAM permissions, they will not be able to interact with the Kubernetes APIs. This creates a challenge in terms of giving others access to the cluster.

EKS normally handles this mapping through a configmap: `kube-system/aws-auth`, but would require you to manually add each user individually, which can be error prone.

To make this more manageable, Konstellation creates an admin role `kon-<cluster>-admin-role` for each cluster that it creates, and assumes this role for all of the interactions it has with Kubernetes.

Each user will still have a unique username when interacting with Kubernetes. Their Kubernetes username would become: `user:<IAM username>`.

## Specifying an admin group

When you create a cluster, Konstellation will ask you for an IAM group that would be mapped to a Kubernetes admin. You can use an existing IAM group or create a new one. For any users that should have access to the Kubernetes cluster, add them to the admin group.

With this approach, as you create new clusters, they could use the same group membership to manage access.
