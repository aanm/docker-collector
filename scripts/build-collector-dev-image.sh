#!/usr/bin/env bash
set -e
dir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

cd "${dir}/.."
mkdir -p ./docker-collector-dev

cp Dockerfile.dev ./docker-collector-dev/Dockerfile
cp -r configs ./docker-collector-dev/configs
cp docker-collector-Linux-x86_64 ./docker-collector-dev

cd ./docker-collector-dev

docker build -t cilium/docker-collector .

cd ..
rm -fr ./docker-collector-dev

echo "Docker collector development image successfully created"

exit 0
