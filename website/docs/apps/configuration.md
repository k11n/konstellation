---
title: Config & Environment
---

App configs are used to define variables that an application requires. In Konstellation, they are kept separate from the main app manifest. Configs are stored as a custom resource in Konstellation and [passed to apps as environment variables](https://12factor.net/config).

The interface for configurations is an YAML file. You can create or edit them in an editor to be saved to Kubernetes with `kon config edit`. Any changes to a config that an app relies on will automatically create a new release. This means that releases are versioned by configs in addition to build changes. This is important since a bad config update could botch a deployment.

There are two kinds of configs: config for a single app, or shared configs. They serve different purposes and can be used together.

To see the configs that are available on the current cluster, use `kon config list`.

### App config

App config is a config that's usable by a single app. When you create an app config, it's made available automatically to the app as environment variables. For more complex config structures, a copy of the config yaml is also set as env var `APP_CONFIG`.

For example, create a config for "myapp" with: `kon config edit --app myapp`.

```yaml title="myapp.yaml"
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

When `myapp` is ran, it'll will automatically receive these environment variables:

| env          | value              |
|:------------ |:------------------ |
| TITLE        | "My website title" |
| PUBLISHED_AT | "1590564232"       |
| APP_CONFIG   | a copy of the config in YAML |

Note that keys have been converted to upper case, and any dashes `-` that you might have in the key names are converted to underscores `_`.

Because the `navigation` field is not a simple scalar value, Konstellation does not attempt to convert it to an env var. Instead, the entire config file is available in the `APP_CONFIG` variable.

### Shared config

While app configs are great way to set app specific configurations, it could lead to duplication when the same configuration is required by multiple apps. For example, you may want to store connection to databases that multiple apps require. Editing each app config would be a massive duplication of effort.

Shared configs is the way to accomplish this. Shared configs are given names, and can be referenced by multiple apps. Below is an example of creating a shared config and using it in my app. Create a shared config with `kon config edit --name db-connection`

```yaml title="db-connection.yaml"
engine: mysql
host: mysql.host.com
user: username
```

Once the config file is created, you can then reference it in the `configs` field in the manifest. `kon app edit <myapp>`

```yaml title="App.yaml"
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

To see the final config values that a specific release of an app will receive, use the `kon config show` command.
