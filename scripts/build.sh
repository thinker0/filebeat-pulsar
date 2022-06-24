#!/bin/bash
set -x
branch=$(git rev-parse --abbrev-ref HEAD)
last_tag=$(git describe --abbrev=0 --tags)
last_commit_id=$(git log --pretty=format:"%h" -1)
flags="-X 'main.Version=${last_tag}' "
flags+="-X 'main.GitTag=${last_tag}' "
flags+="-X 'main.GitCommit=${last_commit_id}' "
flags+="-X 'main.GitTreeState=${branch}' "
flags+="-s -w "

GOOS=linux GOARCH=amd64 go build -ldflags "${flags}" -o filebeat ./cmd/filebeat
