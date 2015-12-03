#!/usr/bin/env bash
dir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

cd "${dir}/.."

deps=(\
"github.com/fsouza/go-dockerclient" \
"github.com/op/go-logging" \
"github.com/samalba/dockerclient" \
"gopkg.in/olivere/elastic.v3" \
)

special_deps=(\
"github.com/docker/docker/..." \
)

echo "Pulling necessary images from DockerHub..."
for dep in "${deps[@]}"; do
    echo "Updating: ${dep}"
    go get -u "${dep}"
    godep update "${dep}"
done

for dep in "${special_deps[@]}"; do
    echo "Updating: ${dep::-4}"
    go get -u "${dep::-4}"
    godep update "${dep}"
done

godep save -r ./...

exit 0
