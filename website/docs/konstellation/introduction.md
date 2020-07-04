---
title: Introduction
---

Konstellation is an open source app platform that helps developers to deploy apps & services quickly on Kubernetes. It's a production-grade system that lets you manage a scalable infrastructure, without having to deal with the layers of complexities that comes with Kubernetes.

## What is it

Konstellation is set of software components working together, including

* a management CLI that runs on your dev machine (`kon`)
* [custom resource definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRDs) that allows you to specify Apps as a first class resource
* a set of best of breed components (Istio, Ingress controller, Prometheus, Grafana) configured to compatible with the version of Kubernetes on the cluster.
* a [Kubernetes operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that makes all of the components work together
* a set of Grafana dashboards for observability into health of your apps
* Terraform based automation that manages cloud resources related to your clusters.

## AWS EKS

The ambition of Konstellation is to provide an abstraction over Kubernetes implementations at various cloud providers so that your apps remain portable. However, we need to start by doing one thing _really_ _really_ _well_. We've decided that the initial target would be Amazon AWS. Konstellation will create and manage clusters on EKS, as well as work with the rest of the AWS ecosystem (load balancers, IAM).

Konstellation itself is designed to be as cloud agnostic as possible, making it simple to plug in additional cloud providers in the future. If you are interested in making this happen, please consider contributing.

## Beta software

Konstellation is currently in beta. As with most beta software, you should expect bugs to be there and be willing to [report them](https://github.com/k11n/konstellation/issues). We'll do our best at addressing incoming issues as quickly as possible.
