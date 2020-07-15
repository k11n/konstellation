---
title: Principles
---

## Stability & cohesion

Konstellation is designed to provide an integrated platform involving multiple OSS components. Thus, one of our core goals is to provide cohesion with all of the included software components.

When upgrading third party components, we choose versions of components that have been tested to work with each other, as well as the version of Kubernetes cluster that you create.

## Abstraction with extensibility

In order to simplify complex systems, we create abstractions as software engineers. We believe the right type of abtractions would make the common case much less painful, but yet does not make it opaque so it becomes impossible access the underlying engine.

Konstellation achieves this with its operator and Custom Resource Definitions. While it drastically simplifies what's involved in creating and managing apps, it does so by managing underlying native Kubernetes resources.

You can access and inspect the Kube resources created by Konstellation, and they will continue to function in absense of the Konstellation operator.

## Reproducibility and undo

With infrastructure changes in the cloud, it can be easy to create a lot of resources that are inter-dependent, making it difficult to reproduce the setup or to remove. Konstellation automates resources management, tracking all of the resources that it creates, and tagging them accordingly. For the cluster and VPC, a `destroy` command would remove everything it allocated.

It should also be easy to replicate a cluster, with no manual steps. Konstellation stores all manifests as Kubernetes resources, and offers export and import commands to easily recreate the same setup.

The same principals also apply to managing app releases. Instead of relying on a traditional Kubernetes Deployment, Konstellation creates distinct AppReleases for just about any changes to apps. This includes build, config, etc. Each App Release can be managed independently, and rolled back if necessary.

## Optimized for apps, not databases

An application typically involves a combination services and databases. While it's possible to run databases inside of Kubernetes, we prefer to run them separately. This is because:

* Databases benefit from having close to the metal access
* Operating databases is very different from operating services, and there are managed services that solve that problem very well. ([RDS](https://aws.amazon.com/rds/), [ElastiCache](https://aws.amazon.com/elasticache/), [ScyllaCloud](https://www.scylladb.com/product/scylla-cloud/) to name a few)
* Scaling databases is tricky, due to the amount of data on disk. The same policies used for scaling homogenous services (autoscaling based on CPU utilization) isn't as effective when scaling DBs.

Konstellation focuses on apps (or services/microservices), and is designed to allow you to point to externally hosted databases via [Configs](apps/configuration.md).

## Upgrading software

Upgrading major components on a live cluster can be unpredictable. I've had multiple instances where a seemingly simple software upgrade would proceed to take down the entire production cluster, causing downtime and major headaches.

Konstellation takes a different strategy to upgrading software components. For the components Konstellation installs onto a cluster, they are frozen at the time of initial installation. They will not be changed or upgraded, in order to optimize for stability.

With new Konstellation releases, they will include updates to the dependent components, and each release will be tested with the components working together in concert. Konstellation users should create new clusters periodically, and load all of the apps configs that are previously installed. Then switch the DNS endpoint of your domains to shift traffic over to the new cluster. The old cluster could then be deprecated and shut down.

## Versions

Konstellation uses [SemVer](https://semver.org/), and will adopt the following guarantees:

* releases within the same __major__ version will be API compatible between the cluster and CLI
* releases within the same __minor__ version will only include components with only minor version changes
