---
title: App Manifest
---

## App.yaml

The main entry point, the app manifest is the single source of truth to how an app should be deployed. This section serves as a reference to the nitty gritty details of the app yaml.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| registry      | string          | no       | Docker registry where your image is hosted at. Defaults to Docker Hub
| image         | string          | yes      | Your app's docker image
| imageTag      | string          | no       | Tag to use for the initial release
| ports         | List[[PortSpec](#portspec)]  | no       | Ports that your app surfaces
| command       | List[string]    | no       | Override for your docker image's ENTRYPOINT
| args          | List[string]    | no       | Arguments to the entrypoint. The docker image's CMD is used if this is not provided.
| configs       | List[string]    | no       | [Shared Configs](apps.md#configuration) that your app needs
| dependencies  | List[[AppReference](#appreference)] | no    | List of other apps your app depends ons, your app will receive their hostnames
| resources     | [ResourceRequirements](#resource-requirements) | no | Define CPU/Memory requests and limits
| scale         | [ScaleSpec](#scalespec) | no | Scaling limits and behavior
| probes        | [ProbeConfig](#probeconfig) | no | Probes to determine app readiness and liveness
| targets       | List[[TargetConfig](#targetconfig)] | yes | Define one or more targets

## AppReference

References an app as a dependency.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| name          | string          | yes      | Name of the app
| target        | string          | no       | Target you are dependent upon, by default, it's the same target as the current running app
| port          | string          | no       | Name of the port you need, when undefined, it references all defined ports.

## IngressConfig

Specification for an Ingress. An Ingress always listens on port 80/443 externally.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| hosts         | List[string]    | yes      | A list of hostnames that the target should run on
| port          | string          | no       | Target port that traffic should be routed to. Defaults to the first defined port.

## PortSpec

Specification for a port

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| name          | string          | yes      | Name of the port. This will be used to reference the port elsewhere
| port          | int             | yes      | Port that your app listens on

## Probe

Probes allows Kubernetes to understand the state of your app so that it could act accordingly.

| Field               | Type            | Required | Description                    |
|:------------------- |:--------------- |:-------- |:------------------------------ |
| httpGet             | HTTPGetAction   | no       | Set if using HTTP probes       |
| exec                | [ExecAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#execaction-v1-core)      | no       | Set if using command line probes
| initialDelaySeconds | int             | no       | Number of seconds after the container has started before probes are initiated.
| timeoutSeconds      | int             | no       | Number of seconds after which the probe times out. Defaults to 1 second.
| periodSeconds       | int             | no       | How often (in seconds) to perform the probe. Default to 10 seconds.
| successThreshold    | int             | no       | Minimum consecutive successes for the probe to be considered successful after having failed. Defaults to 1.
| failureThreshold    | int             | no       | Minimum consecutive failures for the probe to be considered failed after having succeeded. Defaults to 3.

### HTTP Probe Example

```yaml
httpGet:
  path: /running
  port: http
failureThreshold: 3
initialDelaySeconds: 10
timeoutSeconds: 2
```

The probe considers a status code that's not 200 as a failure

### Exec Probe Example

```yaml
exec:
  command:
    - cat
    - /tmp/healthy
failureThreshold: 3
initialDelaySeconds: 10
timeoutSeconds: 2
```

The probe considers a non-zero return code as a failure

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

Example

```yaml
resources:
  requests:
    memory: '100Mi'
    cpu: '1000m'
  limits:
    memory: '200Mi'
    cpu: '1200m'
```

## ScaleSpec

Controls the scaling behavior of your app

| Field                          | Type            | Required | Description                    |
|:------------------------------ |:--------------- |:-------- |:------------------------------ |
| targetCPUUtilizationPercentage | string          | no       | Scale up or down to get to this level of ideal CPU utilization
| min                            | int             | no       | Min number of instances. Default 1
| max                            | int             | no       | Max number of instances. Defaults to same as min

## TargetConfig

Defines for the behavior for the target. The target name must match one of the supported targets in your cluster config in order for the app to be deployed on that cluster.

| Field         | Type            | Required | Description                    |
|:------------- |:--------------- |:-------- |:------------------------------ |
| name          | string          | yes      | Name of the target
| ingress       | [IngressConfig](#ingressconfig) | no | Define an ingress if it should have a load balancer endpoint
| resources     | [ResourceRequirements](#resource-requirements) | no | Override the app's resource requirements
| scale         | [ScaleSpec](#scalespec) | no | Override the app's scaling behavior
| probes        | [ProbeConfig](#probeconfig) | no | Override the app's probes

## Examples

[Minimal example](https://github.com/k11n/konstellation/blob/master/examples/simple/2048.yaml)

[Full example]()
