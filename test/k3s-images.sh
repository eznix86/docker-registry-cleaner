#!/usr/bin/env bash
set -euo pipefail

REGISTRY="localhost:5003"

REPOS=("myapp/api" "myapp/frontend" "myapp/worker")
TAGS=("1.0.0" "1.1.0" "1.2.0" "2.0.0" "2.1.0" "latest" "nightly-20240101" "nightly-20240201" "nightly-20240301")

create_and_push() {
  local registry="$1"
  local repo="$2"
  local tag="$3"

  local image="${registry}/${repo}:${tag}"

  local tmpdir
  tmpdir=$(mktemp -d)
  echo "FROM scratch" > "${tmpdir}/Dockerfile"
  echo "COPY hello.txt /hello.txt" >> "${tmpdir}/Dockerfile"
  echo "Hello from ${repo}:${tag}" > "${tmpdir}/hello.txt"

  echo "  Pushing ${image}"
  docker build -t "${image}" "${tmpdir}" >/dev/null 2>&1
  docker push "${image}" >/dev/null 2>&1
  rm -rf "${tmpdir}"
}

push_untagged() {
  local registry="$1"
  local repo="$2"

  local tmpdir
  tmpdir=$(mktemp -d)
  echo "FROM scratch" > "${tmpdir}/Dockerfile"
  echo "COPY untagged.txt /untagged.txt" >> "${tmpdir}/Dockerfile"
  echo "Untagged v1 for ${repo}" > "${tmpdir}/untagged.txt"

  local image="${registry}/${repo}:temp-untagged"

  docker build -t "${image}" "${tmpdir}" >/dev/null 2>&1
  docker push "${image}" >/dev/null 2>&1

  echo "Untagged v2 for ${repo}" > "${tmpdir}/untagged.txt"
  docker build -t "${image}" "${tmpdir}" >/dev/null 2>&1
  docker push "${image}" >/dev/null 2>&1

  rm -rf "${tmpdir}"
}

echo "=== Pushing images to k3s registry (${REGISTRY}) ==="

for repo in "${REPOS[@]}"; do
  for tag in "${TAGS[@]}"; do
    create_and_push "$REGISTRY" "$repo" "$tag"
  done
done

echo "  Pushing untagged images..."
for repo in "${REPOS[@]}"; do
  push_untagged "$REGISTRY" "$repo"
done

echo "=== All images pushed to k3s registry ==="
