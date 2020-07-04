# kube-prometheus source

this repo contains jsonnet files, to build first install prerequisites

```shell
brew install go-jsonnet kustomize
go get github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb
go get github.com/brancz/gojsontoyaml
```

And build with `./build.py 0.4`
