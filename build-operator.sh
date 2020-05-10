#!/bin/sh

VERSION=0.1.0
NAME="k11n/operator:v$VERSION"

operator-sdk build $NAME
docker push $NAME
