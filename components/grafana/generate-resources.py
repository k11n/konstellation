#!/usr/bin/env python3

import sys
from glob import glob
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
    istio_dir = path.join(target_dir, 'istio')
    kon_dir = path.join(target_dir, 'kon')

    f = open(source, "rb")
    content = yaml.load(f)
    f.close()

    if 'items' not in content or content.get('kind') != 'ConfigMapList':
        raise Exception("File is not a ConfigMapList")

    files = [
        'grafana-instance.yaml',
        'datasource.yaml',
    ]
    for item in content['items']:
        json_name = list(item['data'].keys())[0]
        value = item['data'][json_name]
        filename = generate_dash(target_dir, json_name, value)
        files.append(path.join('dashboards', filename))

    # generate istio json dash
    json_files = glob(path.join(istio_dir, "*.json"))
    json_files.extend(glob(path.join(kon_dir, "*.json")))
    for json_file in json_files:
        f = open(json_file)
        content = f.read()
        f.close()
        # replace datasource
        content = content.replace('"datasource": "Prometheus"',
                                  '"datasource": "prometheus"')
        json_name = path.basename(json_file)
        filename = generate_dash(target_dir, json_name, content)
        files.append(path.join('dashboards', filename))

    # generate kustomization file
    kustomization = {
        "apiVersion": "kustomize.config.k8s.io/v1beta1",
        "kind": "Kustomization",
        "resources": files,
    }

    with open(path.join(path.dirname(__file__), "kustomization.yaml"), "w") as f:
        ruamel.yaml.round_trip_dump(kustomization, f)


def generate_dash(target_dir, json_name, value) -> str:
    name = json_name.split('.')[0]
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
            "name": json_name,
            "json": ruamel.yaml.scalarstring.PreservedScalarString(value),
        }
    }

    with open(target_file, "w") as f:
        ruamel.yaml.round_trip_dump(
            dash, f, default_style=None, default_flow_style=False)

    return path.basename(target_file)

main()
