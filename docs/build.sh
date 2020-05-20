#!/bin/sh

# abort on errors
set -e

# build
yarn build

# navigate into the build output directory
cd .vuepress/dist

# if you are deploying to a custom domain
echo 'konstellation.dev' > CNAME

git init
git add -A
git commit -m 'deploy'

# if you are deploying to https://<USERNAME>.github.io
git push -f git@github.com:k11n/k11n.github.io.git master

cd -
