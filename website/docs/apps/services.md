---
title: Microservices
---

## Connecting to services

Things get slightly trickier when your app relies on other apps (services). The first problem is finding the hostname or URL of the target service(s). While Kubernetes creates service hostnames, it's not desirable to hardcode them because:

* hardcoding hostnames is 🤮
* it's brittle as the service's port could change
* when testing locally, you won't be able to connect to services within the cluster

Konstellation solves this problem by letting apps declare [dependencies](../reference/manifest.md#appreference).

Once declared, Konstellation resolves dependent services for you and places connection host:port into the app's environment. It will also ensure deployment order, so that your app will start only after all of the dependent apps have been created.

:::caution
Konstellation doesn't validate that dependencies actually exist. If your app is dependent on an app that doesn't exist, your app will never start.
:::
