#!/bin/bash

rm kon
go build -i
rice append --exec kon --import-path github.com/davidzhao/konstellation/cmd/kon/utils
