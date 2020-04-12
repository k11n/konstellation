#!/bin/bash

rm kon
find ../.. -name ".terraform" -exec rm -r {} \;
#go build -i && \
#rice append --exec kon --import-path github.com/davidzhao/konstellation/cmd/kon/utils
rice embed-go --import-path github.com/davidzhao/konstellation/cmd/kon/utils
go build -i
