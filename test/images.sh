#!/usr/bin/env bash
set -euo pipefail

REGISTRY_AUTH="localhost:5001"
REGISTRY_NOAUTH="localhost:5002"

AUTH_USER="admin"
AUTH_PASS="admin123"

REPOS=("myapp/api" "myapp/frontend" "myapp/worker" "myapp/base")
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

push_to_registry() {
  local registry="$1"
  local user="${2:-}"
  local pass="${3:-}"
  local label="$4"

  if [ -n "$user" ]; then
    docker login "${registry}" --username="${user}" --password="${pass}" >/dev/null 2>&1
  fi

  echo "Pushing images to ${label} (${registry})..."

  for repo in "${REPOS[@]}"; do
    for tag in "${TAGS[@]}"; do
      create_and_push "$registry" "$repo" "$tag"
    done
  done

  # Push untagged images by pushing twice with same tag (leaves old manifest untagged)
  echo "  Pushing untagged images..."
  for repo in "${REPOS[@]}"; do
    local tmpdir
    tmpdir=$(mktemp -d)
    echo "FROM scratch" > "${tmpdir}/Dockerfile"
    echo "COPY untagged.txt /untagged.txt" >> "${tmpdir}/Dockerfile"
    echo "Untagged v1 for ${repo}" > "${tmpdir}/untagged.txt"

    local image="${registry}/${repo}:temp-untagged"

    docker build -t "${image}" "${tmpdir}" >/dev/null 2>&1
    docker push "${image}" >/dev/null 2>&1

    # Overwrite with new content, leaving old manifest untagged
    echo "Untagged v2 for ${repo}" > "${tmpdir}/untagged.txt"
    docker build -t "${image}" "${tmpdir}" >/dev/null 2>&1
    docker push "${image}" >/dev/null 2>&1

    rm -rf "${tmpdir}"
  done

  echo "Done pushing to ${label}."
}

echo "=== Docker Registry Cleaner - Test Image Pusher ==="
echo ""

push_to_registry "$REGISTRY_AUTH" "$AUTH_USER" "$AUTH_PASS" "registry-auth"
echo ""
push_to_registry "$REGISTRY_NOAUTH" "" "" "registry-noauth"

echo ""
echo "=== All images pushed ==="
echo "Registry auth: ${REGISTRY_AUTH} (user: admin, pass: admin123)"
echo "Registry noauth: ${REGISTRY_NOAUTH}"
echo ""
echo "Repos: ${REPOS[*]}"
echo "Tags per repo: ${TAGS[*]}"
