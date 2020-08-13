#!/usr/bin/env sh

# abort on errors
set -e

# build
yarn docs:build

# navigate into the build output directory
cd docs/.vuepress/dist

# deploying to a custom domain
echo 'docs.scrt.network' > CNAME

git add -A
git commit -m 'deploy'

cd -