---
title: Introduction
---

Konstellation is an open source application platform that helps developers to deploy and manage apps on Kubernetes. It provides a scalable infrastructure, give you a simple, and yet robust, interface on top of Kubernetes.

## What is it

Konstellation is a complete software stack working on top of Kubernetes. It consists of

* a management CLI that runs on your dev machine (`kon`)
* [custom resource definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRDs) that allows you to specify Apps as a resource
* a set of best of breed components (Istio, Ingress controller, Prometheus, Grafana) configured to compatible with the version of Kubernetes on the cluster.
* a [Kubernetes operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that makes all of the components work together
* Grafana dashboards for observability into health of apps
* Terraform based automation that creates and manages clusters and other cloud resources.

Together, it's designed for you to deploy resilient, scalable apps and services; making it simple to achieve many of the patterns described in [the twelve factor app](https://12factor.net/).

We'll be linking to the best practices throughout this documentation.

## How it works

![How it works](/img/how-it-works.svg)

## AWS EKS

The ambition of Konstellation is to provide an abstraction over Kubernetes implementations across cloud providers so that apps remain portable. However, we need to start by doing one thing _really_ _really_ _well_. We've decided that the initial target would be Amazon AWS. Konstellation will create and manage clusters on EKS, as well as work with the rest of the AWS ecosystem (load balancers, IAM).

Konstellation itself is designed to be as cloud agnostic as possible, making it simple to plug in additional cloud providers in the future. If you are interested in making this happen, please consider contributing.

## Private beta

Konstellation is currently in private beta. If you are interested in trying it during the beta, [sign up here](https://forms.gle/Eh9je8GmS7NRSXf69).
