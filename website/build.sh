#!/bin/sh

# abort on errors
set -e

# build
yarn run build

cd build

# if you are deploying to a custom domain
echo 'konstellation.dev' > CNAME

git init
git add -A
git commit -m 'deploy'

# if you are deploying to https://<USERNAME>.github.io
git push -f git@github.com:k11n/k11n.github.io.git master

cd -
