# App.yaml

The main entry point, the app config is the single source of truth to how an app should be deployed. This section serves as a reference to the nitty gritty details of the app yaml.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| registry      | string          | no       | Docker registry where your image is hosted at. Defaults to Docker Hub
| image         | string          | yes      | Your app's docker image
| imageTag      | string          | no       | Tag to use for the initial release
| ports         | List[[PortSpec](#portspec)]  | no       | Ports that your app surfaces
| command       | List[string]    | no       | Override for your docker image's ENTRYPOINT
| args          | List[string]    | no       | Arguments to the entrypoint. The docker image's CMD is used if this is not provided.
| configs       | List[string]    | no       | [Shared Configs](apps.md#configuration) that your app needs
| dependencies  | List[[AppReference](#appreference)] | no    | List of other apps your app depends ons
| resources     | [ResourceRequirements](#resource-requirements) | no | Define CPU/Memory requests and limits
| scale         | [ScaleSpec](#scalespec) | no | Scaling limits and behavior
| probes        | [ProbeConfig](#probeconfig) | no | Probes to determine app readiness and liveness
| targets       | List[[TargetConfig](#targetconfig)] | yes | Define one or more targets

## AppReference

placeholder

## IngressConfig

## PortSpec

placeholder

## Probe

Probes allows Kubernetes to understand the state of your app so that it could act accordingly.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| httpGet       | HTTPGetAction

## ProbeConfig

ProbeConfig is a container for probe definitions.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| liveness      | [Probe](#probe) | no       | Determines if your app is still running, when this probe fails, Kubernetes will restart your app
| readiness     | [Probe](#probe) | no       | Determines if your app is ready to serve traffic. Sometimes an app may need to load large amount of data before it's ready to serve traffic. An app that isn't reporting it's ready will not receive traffic

Example

```yaml
probes:
  liveness:
    httpGet:
      path: /running
      port: http
    failureThreshold: 3
  readiness:
    httpGet:
      path: /ready
      port: http
    failureThreshold: 3
```

## Resource Requirements

Identical to [Kubernetes Resource Requirements](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

## ScaleSpec

Placeholder

## TargetConfig

Defines for the behavior for the target. The target name must match one of the supported targets in your cluster config in order for the app to be deployed on that cluster.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| name          | string          | yes      | Name of the target
| ingress       | [IngressConfig](#ingressconfig) | no | Define an ingress if it should have a load balancer endpoint
| resources     | [ResourceRequirements](#resource-requirements) | no | Override the app's resource requirements
| scale         | [ScaleSpec](#scalespec) | no | Override the app's scaling behavior
| probes        | [ProbeConfig](#probeconfig) | no | Override the app's probes
