---
title: Microservices
---

## Connecting to services

Things get slightly trickier when an app relies on other apps (services). The first problem is finding the hostname or URL of the target service(s). While Kubernetes creates service hostnames, it's not desirable to hardcode them because:

* hardcoding hostnames is 🤮
* it's brittle as the service's port could change
* when testing locally, you won't be able to connect to services within the cluster

Konstellation solves this problem by letting apps declare [dependencies](../reference/manifest.md#appreference).

Once declared, Konstellation resolves dependent services for you and places connection host:port into the app's environment. It will also ensure deployment order, so that the app will start only after all of its dependencies have been created.

:::caution
Konstellation doesn't validate that dependencies actually exist. If the app is dependent on an app that doesn't exist, it will never start.
:::

As an example

```yaml title="app.yaml"
...
dependencies:
  - name: reviews
    port: grpc
  - name: orders
    port: http
```

This app will receive the following env vars. They can be used to connect to the respective services.

```bash
REVIEWS_GRPC_HOST=reviews.target.svc.cluster.local:3001
ORDERS_HTTP_HOST=orders.target.svc.cluster.local:80
```

When [running locally](/develop#runninglocally), the same environment variables are made available. Since they cannot access private Kubernetes addresses, Konstellation will automatically set up local proxies to your dependencies, then setting the same env vars to the proxy addresses.
