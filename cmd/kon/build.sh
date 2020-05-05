#!/bin/bash

find ../.. -name ".terraform" -exec rm -r {} \;
rice embed-go --import-path github.com/davidzhao/konstellation/cmd/kon/utils
go build -i
