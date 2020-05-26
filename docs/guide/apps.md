# Working with Apps

## Targets

Target is a concept in Konstellation that provides a namespace for your app. The goal of having this layer is to enable you to run different environments for the same app. For example, you could run a production and a development environment, with different configurations for number of instances to run, and hostnames.

Your cluster should define the targets that it supports. This allows you the flexibility of using the same `app.yaml` across multiple clusters, if you choose to have different targets across clusters.

Attributes for a target

| Attribute | Type              | Required | Description             |
| --------- | ----------------- |:--------:|:----------------------- |
| name      | string            | yes      | Name of the target. It must match what's defined in your cluster config. |
| ingress   |                   | no       | Specifies how to expose your app to the outside |
| resources |                   | no       | Target specific resources requirements/limits   |
| scale     |                   | no       | Target specific scaling definition              |
| probes    |                   | no       | Target specific probes |

Most target attributes can be defined on the app itself, and when running under that target, they are inherited from the base config. You may choose to override only specific portion of the attributes, and the result would be merged. The only attribute that's target-specific is `ingress`. Since ingress is specific to hostnames and exposing traffic to the outside world, it must be defined under the target.

App.yaml example:

```yaml
apiVersion: k11n.dev/v1alpha1
kind: App
metadata:
  name: myapp
spec:
  image: repo/myapp
  imageTag: v10
  ports:
    - name: http
      port: 80
  scale:
    targetCPUUtilizationPercentage: 60
    min: 1
    max: 10
  targets:
    - name: staging
      ingress:
        hosts:
          - staging.myapp.com
        port: 80
      scale:
        max: 1
    - name: production
      ingress:
        hosts:
          - www.myapp.com
        port: 80
      scale:
        min: 5
        max: 20
```

In this example, the yaml defines two targets, `staging` and `production`. Note that we override the scale attribute for each target. With the overrides applied, `staging`'s scale attributes would become:

```yaml
targetCPUUtilizationPercentage: 60
min: 1
max: 1
```

`production` scale would be:

```yaml
targetCPUUtilizationPercentage: 60
min: 5
max: 20
```

## Releases

A release is a base unit of an app's deployment. It locks in your app's build along with any configs that are associated with the app. Each change in your app's build or app config would trigger a new release to be created. You could list the releases with `kon app status <yourapp>`

To deploy a new build, use `kon app deploy --tag <docker tag> --app <yourapp>`

Konstellation would scale up your release over a period of time, and slowly shift traffic over to the new release. If there's a problem with a particular build or configuration, you could rollback to a prior working release with the `kon app rollback` command. Rollback marks a particular release as bad, and will cause the system to automatically deploy the previous working version.

## Configuration

Configurations are used to contain variables or attributes that your application requires. They are kept separate from the main app.yaml definition as they are likely to change. Configs are stored as a custom resource in Konstellation and passed to y our app as environment variables. Any changes to a config that your app relies on will automatically create a new release. They are editable via the `kon` CLI.

There are two types of configs: configs for a single app, or shared config files. They could be used together.

### App config

App config is a config that's usable by a single app. It's loaded by the app automatically with its keys as environment variables. For more complex config structures, a copy of the config yaml is also set as an env var.

### Shared config

### Config for a target

You could have configurations specific to a target.

### Config and releases

Every change your app's config (including shared configs that your app depends on) will generate a new release.

## Running Locally

## Deploying Updates

## Rolling back
