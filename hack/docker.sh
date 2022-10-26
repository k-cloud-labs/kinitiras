#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script holds docker related functions.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

REGISTRY=${REGISTRY:-"registry.cn-hangzhou.aliyuncs.com/k-cloud-labs"}
VERSION=${VERSION:="unknown"}
DockerfileName=${DockerfileName:="Dockerfile"}

function build_images() {
  local target="$1"
  docker build -t ${REGISTRY}/kinitiras-${target}:${VERSION} -f ${REPO_ROOT}/images/${target}/${DockerfileName} ${REPO_ROOT}
}

build_images $@