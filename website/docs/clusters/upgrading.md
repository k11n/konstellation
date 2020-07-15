---
title: Upgrading
---

## Create and migrate

A cluster contains many moving parts. When new versions of these parts are released, it's not always guaranteed that they will function as expected with other components in the system.

For this reason, we discourage from upgrading system software on Konstellation clusters while they are handling production traffic. Instead, a better strategy would be to create & migrate:

1. Create a new cluster with the latest software
1. Import your current cluster's configurations
1. Add a new host to your ingress and map that DNS entry
1. Test the new cluster setup to ensure everything is working correctly
1. Point DNS entry to the new load balancer

## Export and import

### Export

`kon cluster export --dir <directory>` would export a copy of Konstellation resources that you have defined. The exported resources include:

* App manifest
* App and shared Configs
* Linked service accounts
* Builds

Resources that are managed by Konstellation operator are not exported. The operator on the new cluster will regenerate them. Since Kubernetes is still changing its API versions, and periodically deprecates older API versions, having this layer of abstraction helps to maintain compatibility across clusters.

It's a good idea to keep a back up of the exported resources, some would even check it into a git repo to keep track of changes across versions.

Other resources that you create separately from Konstellation are also not exported. You can dump a copy of them separately with kubectl

### Import

On a new cluster, you can import all of the exported resources to recreate the state of your apps. `kon cluster import --dir <directory>`.

Once imported, apps would begin to deploy themselves, starting at the min instances as defined in the app manifest.

## Upgrading nodepool

As your cluster grows in utilization, the initial instance size & count on the nodepool may be insufficient to handle the load. To resize the nodepool, the simplest approach is to create & migrate:

1. `kon nodepool create` to create a new nodepool with desired preferences
1. `kon nodepool destroy --nodepool <nodepool name>` on the existing nodepool

When destroy is issued, Konstellation will issue a delete request to EKS on that nodegroup. EKS will first [drain all of the pods](https://docs.aws.amazon.com/eks/latest/userguide/delete-managed-node-group.html) running on the group, causing them to be relocated to the newer nodepool.

## Reinstalling Konstellation

In certain cases, it may be desirable to update to the latest version of Konstellation and bring up to date all of the components running on the cluster. This is not recommended on a production cluster.

```
% kon cluster reinstall
```
