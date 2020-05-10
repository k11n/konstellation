#!/bin/bash

find ../.. -name ".terraform" -exec rm -r {} \;
rice embed-go --import-path github.com/k11n/konstellation/cmd/kon/utils
go build -i
