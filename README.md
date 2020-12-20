# filebeat-pulsar

# Copy of https://github.com/streamnative/pulsar-beat-output.git

# build

```bash
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o filebeat ./cmd/filebeat
```