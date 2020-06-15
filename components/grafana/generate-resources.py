#!/usr/bin/env python3

import sys
from os import path

import ruamel.yaml
from ruamel.yaml import YAML


__namespace__ = "grafana"
yaml = YAML(typ='safe')


def main():
    if len(sys.argv) == 1:
        print("File is expected")
        exit(1)

    source = sys.argv[1]
    target_dir = path.join(path.dirname(__file__), "dashboards")

    f = open(source, "rb")
    content = yaml.load(f)
    f.close()

    if 'items' not in content or content.get('kind') != 'ConfigMapList':
        raise Exception("File is not a ConfigMapList")

    files = [
        'grafana-instance.yaml',
        'dashboards/datasource.yaml',
    ]
    for item in content['items']:
        filename = generate_dash(target_dir, item)
        files.append(path.join('dashboards', filename))

    # generate kustomization file
    kustomization = {
        "apiVersion": "kustomize.config.k8s.io/v1beta1",
        "kind": "Kustomization",
        "resources": files,
    }

    with open(path.join(path.dirname(__file__), "kustomization.yaml"), "w") as f:
        ruamel.yaml.round_trip_dump(kustomization, f)


def generate_dash(target_dir, item) -> str:
    key_name = list(item['data'].keys())[0]
    value = item['data'][key_name]
    name = key_name.split('.')[0]
    target_file = path.join(target_dir, name + ".yaml")

    dash = {
        "apiVersion": "integreatly.org/v1alpha1",
        "kind": "GrafanaDashboard",
        "metadata": {
            "name": name,
            "namespace": __namespace__,
            "labels": {
                "app": "grafana",
            }
        },
        "spec": {
            "name": key_name,
            "json": ruamel.yaml.scalarstring.PreservedScalarString(value),
        }
    }

    with open(target_file, "w") as f:
        ruamel.yaml.round_trip_dump(
            dash, f, default_style=None, default_flow_style=False)

    return path.basename(target_file)

main()
