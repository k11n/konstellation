# kube-prometheus source

this repo contains jsonnet files, to build first install prerequisites

```shell
brew install go-jsonnet kustomize
go get github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb
go get github.com/brancz/gojsontoyaml
```

Then init jsonnet deps

```
jb install
```

And finally build with `./build.sh`
