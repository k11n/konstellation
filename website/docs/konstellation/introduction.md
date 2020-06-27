---
title: Introduction
---

Konstellation is an open source app platform that helps you deploy your apps & services quickly on Kubernetes. It's designed to be a production-scale system that lets you harness the power of Kubernetes and yet being as easy to operate as Heroku.

## What is it

Konstellation is an integration of original and third party open source components. It includes

* a management CLI that runs on your dev machine (`kon`)
* [custom resource definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRDs) that allows you to specify Apps as a first class resource
* a set of best of breed components (istio, ingress controller, prometheus, grafana) configured to compatible with the version of Kubernetes on the cluster.
* a [Kubernetes operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that makes all of the components work together
* Terraform based automation that manages cloud resources related to your clusters.

Working together, it allows a developer to focus the outcome that you want to achieve, versus the nitty gritty of how to achieve that outcome in a Kubernetes world.

## AWS EKS

The ambition of Konstellation is to provide an abstraction over Kubernetes implementations at various cloud providers so that your apps remain portable. However, we need to start by doing one thing _really_ _really_ _well_. We've decided that the initial target would be Amazon AWS. Konstellation will create and manage clusters on EKS, as well as work with the rest of the AWS ecosystem (load balancers, IAM).

Konstellation itself is designed to be as cloud agnostic as possible, making it simple to plug in additional cloud providers in the future. If you are interested in making this happen, please consider contributing.

## Beta software

Konstellation is currently in beta. As with most beta software, you should expect bugs to be there and be willing to [report them](https://github.com/k11n/konstellation/issues). We'll do our best at addressing incoming issues as quickly as possible.
