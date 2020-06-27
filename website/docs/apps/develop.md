---
title: Development & Debugging
---

One of the shortcomings of Kubernetes is that it's hard to know what's going on within the cluster. It could sometimes feel like a blackbox, with limited tools in seeing what's in there. And if your app is failing in production, how do you replicate the setup to reproduce the error locally?

Konstellation simplifies the debugging & development process by giving you an app-centric way of

## Getting logs

To pull your apps logs from production, or to tail the logs, you would run the `kon app logs <yourapp>` command. It will guide you through the process of picking a pod to inspect.

By default, it'll print the last 100 lines of logs from your container. To follow logs, run `kon app logs -f <yourapp>`.

## Running locally

When you need to replicate the in-cluster setup to test your app locally, Konstellation provides a shortcut in doing so.

It has a `local` mode that replicates the in-cluster config for your app. To use this, run `kon app local [--target target] <yourapp>`

If a target is not passed in, it'll use the first target defined.

When running locally, the same environment variables will be set for the configuration and hostnames of service dependencies. For service dependencies, proxies will be created on localhost that would allow you to connect to services running inside of the Kubernetes cluster. This is extremely useful in an microservices environment since you could avoid replciating the entire setup in order to test a single service.

## Inspecting the container

You can get shell access to any instances of your app with `kon app shell <yourapp>`.

In order for this to work, you need to have a shell installed on the docker image. By default Konstellation will launch `/bin/sh`. To override the shell, run `kon app shell --shell <path-to-shell> <yourapp>`