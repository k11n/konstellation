---
title: Managing Users
---

When an EKS cluster is setup, by default it grants permissions to only the IAM user that created the cluster. Even when other users have the appropriate IAM permissions, they will not be able to interact with the Kubernetes APIs and therefore Konstellation won't work for other users on your team.

EKS handles this mapping through a configmap: `kube-system/aws-auth`

To make this more manageable, Konstellation creates an admin role `kon-<cluster>-admin-role` for each cluster that it creates, and assumes this role for all of the interactions it has with Kubernetes.

Each user will still have a unique username when interacting with Kubernetes. Their kubernetes username would be: `user:<IAM username>`.

## Specifying an admin group

When you create a cluster, Konstellation will ask you for an IAM group that would be mapped to a Kubernetes admin. You can use an existing IAM group or create a new one. For any users that should have access to the Kubernetes cluster, add them to the admin group.

With this approach, as you create new clusters, they could use the same group membership to manage access.
