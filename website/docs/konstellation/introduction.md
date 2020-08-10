---
title: Introduction
---

Konstellation is an open source application platform that helps developers to deploy and manage apps on Kubernetes. It provides a scalable infrastructure, give you a simple, and yet robust interface on top of Kubernetes.

## What is it

Konstellation is a complete software stack working on top of Kubernetes. It consists of

* a management CLI that runs on your dev machine (`kon`)
* [custom resource definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRDs) that allows you to specify Apps as a resource
* a set of best of breed components (Istio, Ingress controller, Prometheus, Grafana) configured to compatible with the version of Kubernetes on the cluster.
* a [Kubernetes operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that makes all of the components work together
* Prometheus operator configured to scrape from k8s, istio, as well as any apps
* Grafana dashboards for observability
* Terraform based automation that creates and manages clusters and other cloud resources.

Together, it's designed for you to deploy resilient, scalable apps and services; making it simple to achieve many of the patterns described in [the twelve factor app](https://12factor.net/).

We'll be linking to the best practices throughout this documentation.

## How it works

![How it works](/img/how-it-works.svg)

## AWS EKS

The ambition of Konstellation is to provide an abstraction over Kubernetes implementations across cloud providers so that apps remain portable. However, we need to start by doing one thing _really_ _really_ _well_. We've decided that the initial target would be Amazon AWS. Konstellation will create and manage clusters on EKS, as well as work with the rest of the AWS ecosystem (load balancers, IAM).

Konstellation itself is designed to be as cloud agnostic as possible, making it simple to plug in additional cloud providers in the future. If you are interested in making this happen, please consider contributing.

## Third-party software and licensing

Konstellation is Apache 2.0 licensed. It makes of use of other Apache 2.0 licensed software. When a cluster is created with Konstellation, the following components will be installed onto your cluster:

* [ALB Ingress Controller](https://github.com/kubernetes-sigs/aws-alb-ingress-controller)
* [Grafana Operator](https://github.com/integr8ly/grafana-operator)
* [Grafana](https://github.com/grafana/grafana)
* [Istio](https://istio.io/)
* [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler)
* [Kubernetes Dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/)
* [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
* [Prometheus Operator](https://github.com/coreos/prometheus-operator)
* [Prometheus](https://prometheus.io/)

## Support Slack

[Join Slack](https://join.slack.com/t/kon-users/shared_invite/zt-fm64885u-QSlL0VQUJdZ_rcQBaZ81ug) if you  are interested in using Konstellation and have questions or feedback.
