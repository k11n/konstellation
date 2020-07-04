#!/usr/bin/env python3

import glob
import os
import shutil
import sys
from os import path

__exclude_files__ = [
    'prometheus-prometheus.yaml',
    'grafana-',
]


def main():
    if len(sys.argv) == 1:
        print("Usage: %s <version>" % sys.argv[0])
        exit(1)
    version = sys.argv[1]
    basedir = path.abspath(path.join(path.dirname(__file__), version))
    jsonnet = 'main.jsonnet'

    os.chdir(basedir)
    shutil.rmtree("build", ignore_errors=True)
    os.makedirs("build/setup")
    os.makedirs("dist", exist_ok=True)
    # Calling gojsontoyaml is optional, but we would like to generate yaml, not json
    cmd = r"jsonnet -J vendor -m build %s | xargs -I{} sh -c 'cat {} | gojsontoyaml > {}.yaml' -- {}" % jsonnet
    print(cmd)
    os.system(cmd)

    # copy all other premade files
    other_files = glob.glob(path.join(basedir, '..', 'kon', '*'))

    for item in other_files:
        shutil.copyfile(item, path.join(basedir, "build", path.basename(item)))

    # now go into build dir, and create
    process_dir(path.join(basedir, "build", "setup"),
                path.join(basedir, "dist", "prometheus-operator.yaml"))
    process_dir(path.join(basedir, "build"),
                path.join(basedir, "dist", "prometheus-k8s.yaml"))


def process_dir(dir: str, target_file: str):
    odir = os.getcwd()
    os.chdir(dir)
    try:
        yaml_files = []
        for item in glob.glob("*"):
            if path.isdir(item):
                continue
            should_exclude = False
            for exclude in __exclude_files__:
                if item.startswith(exclude):
                    should_exclude = True
                    break

            if should_exclude:
                continue
            if not item.endswith(".yaml"):
                # delete
                os.remove(item)
                continue
            yaml_files.append(item)
        yaml_files.sort()

        # generate kustomize
        kustomization = """apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
"""
        for item in yaml_files:
            kustomization += "- %s\n" % item

        with open("kustomization.yaml", "w") as f:
            f.write(kustomization)

        # run kustomize
        os.system("kustomize build . > %s" % target_file)
    finally:
        os.chdir(odir)


main()
