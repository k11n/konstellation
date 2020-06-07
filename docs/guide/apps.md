# Working with Apps

## Container Registry

Your app needs to be hosted by a container registry in order to be used with Kubernetes. The `registry` field in the app manifest specifies the URL to the registry. When `registry` is left blank, it defaults to Docker Hub. By default, the cluster only has access to public repos on Docker Hub. To access private registries, there are two options.

### ECR

The simplest option is to use [Amazon ECR](https://aws.amazon.com/ecr/). When Konstellation sets up the IAM role for the cluster, it included read-only access to your repos on ECR.

Once your images are pushed to ECR, set the registry field your ECR URL. That's it.

## Targets

Target is a concept in Konstellation that provides a namespace for your app. A target is roughly equivalent to a specific environment of your app. For example, you could specify a production and a development target, with different configurations for hostnames, scale parameters, and receive target-specific configs.

Your cluster would define the targets that it supports with `kon cluster configure`. When deploying an app to a cluster, it will set up a deployment for each target that your cluster has declared. This configuration allows for the flexibility of using the same `app.yaml` across multiple clusters, if you prefer to have dedicated clusters for each target.

Most target attributes can be defined on the app itself, and when running under that target, they are inherited from the base config. You may choose to override only specific portion of the attributes, and the result would be merged. The only attribute that's target-specific is `ingress`. Since ingress is specific to hostnames and exposing traffic to the outside world, it must be defined under the target.

Konstellation will automatically create a namespace for each target and place all of the native resources under that namespace.

See [TargetConfig](manifest.md#targetconfig)

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
        port: http
      scale:
        max: 1
    - name: production
      ingress:
        hosts:
          - www.myapp.com
        port: http
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

Configurations are used to contain variables or attributes that your application requires. They are kept separate from the main app.yaml definition. Configs are stored as a custom resource in Konstellation and passed to your app as environment variables.

The interface for configurations is an YAML file. You can create or edit them in an editor to be saved backed to Kubernetes with `kon config edit`. Any changes to a config that your app relies on will automatically create a new release. This means that releases are versioned by configs in addition to build changes, since a bad config update could botch your app.

There are two types of configs: configs for a single app, or shared config files. They serve different purposes and can be used together.

To see the configs that are available on the current cluster, use `kon config list`.

### App config

App config is a config that's usable by a single app. When you create an app config, it's made available automatically to the app as environment variables. For more complex config structures, a copy of the config yaml is also set as env var `APP_CONFIG`.

For example, if you created a config for your app: `kon config edit --app myapp`

```yaml
title: My website title
published_at: 1590564232
navigation:
  sidebar:
    - hello
    - world
  topnav:
    - link1
    - link2
```

When executed, your app will receive these environment variables:

| env          | value              |
|:------------ |:------------------ |
| TITLE        | "My website title" |
| PUBLISHED_AT | "1590564232"       |
| APP_CONFIG   | a copy of the config in YAML |

Note that keys have been converted to upper case, and any dashes `-` that you might have in the key names are converted to underscores `_`.

Because the `navigation` field is not a simple type, Konstellation does not attempt to convert it to an env var. Instead, the entire config file is available in the `APP_CONFIG` variable.

### Shared config

While app configs are great way to pass app specific configuration to your app, what about for configurations that multiple apps care about? For example, you may want to store connection to databases without having to modify multiple app configs to pass along the same information.

Shared configs is the way to accomplish this. Shared configs are given names, and can be referenced by multiple apps. Below is an example of creating a shared config and using it in my app. Create a shared config with `kon config edit --name db-connection`

db-connection shared config

```yaml
engine: mysql
host: mysql.host.com
user: username
```

Once the config file is created, you can then reference it in your app's `configs` field. `kon app edit <myapp>`

App.yaml

```yaml
apiVersion: k11n.dev/v1alpha1
kind: App
metadata:
  name: myapp
spec:
  image: repo/myapp
  configs:
    - db-connection
  targets:
    - name: production
```

Save the app.yaml file and a new release will be created that passes it a new environment variable `DB_CONNECTION`, with the value being set to the contents of the db config in YAML.

### Target specific overrides

In certain cases, it's desirable to have certain config attributes to differ between the different environments. For example, you may have a staging database and a production one. Konstellation offers a away to define target specific overrides.

When you edit a config by passing in a `--target` flag, it will create an override file where those values only apply to that specific target. At run time, all of the values you've defined for the target would be merged into the base config.

To see the final config values that a specific release of your app will receive, use the `kon config show` command.

## Running locally

To make testing locally more convenient, Konstellation has a `local` mode that replicates the production config for your app. To use this, run `kon app local [--target target] <yourapp>`

If a target is not passed in, it'll use the first target defined.

When running locally, the same environment variables will be set for the configuration and hostnames of service dependencies. For service dependencies, proxies will be created on localhost that would allow you to connect to services running inside of the Kubernetes cluster.

## Deploying new builds

When a new docker image of your app is built, you will need to tell Konstellation of that new build with command `kon app deploy`

This command will deploy your app to all targets simultenously.

## Rolling back

## Autoscaling

