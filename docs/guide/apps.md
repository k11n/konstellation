# Working with Apps

## Targets

Target is a concept in Konstellation that provides a namespace for your app. The goal of having this layer is to enable you to run different environments for the same app. For example, you could run a production and a development environment, with different configurations for number of instances to run, and hostnames.

## Releases

## Configuration

Configs are a native type in Konstellation. You may edit configs as YAML files, and then they are passed to your app(s) as environment variables

There are two types of configs: configs for a single app, or shared config files. They could be used together.

### App Config

App config is a config file for a single app. They work almost

### Config for a target


### Config and releases

Every change your app's config (including shared configs that your app depends on) will generate a new release.

## Running Locally

## Deploying Updates

## Rolling back
