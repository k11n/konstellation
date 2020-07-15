---
title: Development & Debugging
---

One of the shortcomings of Kubernetes is that it can be difficult to know what's going on within the cluster. It sometimes feels like a blackbox, with limited tools in seeing what's in there. And if your app is failing in production, how do you replicate the setup to reproduce the error locally?

Konstellation simplifies the debugging & development process by giving you an app-centric tools to obtain visibility.

## Logging

Apps should [write logs to stdout](https://12factor.net/logs), in JSON. It should not have to worry about the underlying storage of logs, or be concerned with uploading logs to another service. Konstellation works with Kubernetes logging to perform standard log operations.

To pull app logs from production, or to tail the logs, you would run the `kon app logs <yourapp>` command. It will guide you through the process of picking a pod to inspect.

By default, it'll print the last 100 lines of logs from your container. To follow logs, run `kon app logs -f <yourapp>`.

For more advanced log management, you could use third party solutions that integrate with Kubernetes, such as [Fluentd](https://docs.fluentd.org/container-deployment/kubernetes), [Datadog](https://docs.datadoghq.com/integrations/kubernetes/), or [Sematext](https://sematext.com/docs/agents/sematext-agent/kubernetes/installation/), to name a few.

## Proxy

When apps are running inside Kubernetes, they are typically behind various security groups and inaccessible from developer machines. It's helpful to be able to load the responses manually to inspect it. Kubernetes has a [built-in proxy](https://kubernetes.io/docs/tasks/extend-kubernetes/http-proxy-access-api/) that can map of a cluster address to localhost.

Konstellation integrates with the proxy and makes it easier to reach your apps. `launch proxy` command would open a new proxy to a specific app, mapping it to the app's usual port onto localhost.

```
% kon launch proxy --app myapp
Proxy to production.apiserver: started on http://localhost:9000
```

You can also proxy to non-Konstellation services, by passing in `--service` and `--namespace` flags.

## Running locally

When you need to replicate the in-cluster setup to run an app locally, Konstellation provides a shortcut in doing so.

It has a `local` mode that replicates the in-cluster config for the app. To use this, run `kon app local [--target target] <yourapp>`

If a target is not passed in, it'll use the first target defined.

When running locally, the same environment variables will be set for the configuration and hostnames of service dependencies. For service dependencies, proxies will be created on localhost that would allow you to connect to services running inside of the Kubernetes cluster. This is extremely useful in an microservices environment since you could avoid replicating the entire setup in order to test a single service.

## Inspecting the container

You can get shell access to any instances of an app with `kon app shell <yourapp>`.

In order for this to work, you need to have a shell installed on the docker image. By default Konstellation will launch `/bin/sh`. To override the shell, run `kon app shell --shell <path-to-shell> <yourapp>`
