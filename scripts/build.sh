#!/bin/bash
set -x
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o filebeat ./cmd/filebeat
