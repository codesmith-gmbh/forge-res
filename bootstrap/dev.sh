#!/usr/bin/env bash

exec docker run -it --rm \
    -v /Users/stan:/root \
    -e AWS_PROFILE=codesmith \
    --entrypoint /bin/bash \
    aws/codebuild/golang:1.11