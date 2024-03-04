#!/bin/bash

mkdir -p ./build

# Build
CGO_ENABLED=0 GOOS=linux go build -o ./build/neuralnexus-api

# Docker
# docker build -t p0t4t0sandwich/neuralnexus:api .
# docker push p0t4t0sandwich/neuralnexus:api
