# Docker Registry Cleaner

Docker Registry Cleaner helps you clean up your stale images with retention policies and use your registry garbage collection. It can be used with multiple registries. Works with Kubernetes and Docker/Podman.

## Features

- Multiple registries in a single run
- Per-repository retention policies with tag filters
- Garbage collection for Kubernetes and Docker/Podman
- Dry run mode

## Quick Start

```yaml
# config.yaml
registries:
  - name: my-registry
    url: http://registry:5000
    keep: 5
    delete_untagged: true
    repos:
      my-app:
        keep: 10
        tag_filter: "^v\\d+\\.\\d+\\.\\d+$"
```

```bash
drc -c config.yaml
```

## Configuration

### Global Settings

| Field         | Type     | Default | Description                    |
| ------------- | -------- | ------- | ------------------------------ |
| `log_level`   | string   | `info`  | Log level (debug, info, warn)  |
| `dry_run`     | bool     | `false` | Preview deletions only         |
| `concurrency` | int      | `1`     | Parallel registry processing   |
| `timeout`     | duration | `30s`   | Per-registry timeout           |

### Registry

| Field              | Type   | Required | Description                          |
| ------------------ | ------ | -------- | ------------------------------------ |
| `name`             | string | Yes      | Registry identifier                  |
| `url`              | string | Yes      | Registry URL                         |
| `username`         | string | No       | Basic auth username                  |
| `password`         | string | No       | Basic auth password                  |
| `username_env`     | string | No       | Env var for username                 |
| `password_env`     | string | No       | Env var for password                 |
| `keep`             | int    | Yes      | Default tags to keep per repository  |
| `delete_untagged`  | bool   | No       | Delete manifests without tags        |

### Per-Repository Overrides

| Field          | Type   | Description                          |
| -------------- | ------ | ------------------------------------ |
| `keep`         | int    | Override default tag count           |
| `tag_filter`   | string | Regex to include matching tags only  |
| `delete_untagged` | bool | Override untagged deletion           |
| `skip`         | bool   | Skip this repository entirely        |

### Garbage Collection

#### Kubernetes

| Field                       | Type   | Required | Description                              |
| --------------------------- | ------ | -------- | ---------------------------------------- |
| `kubernetes.enabled`        | bool   | Yes      | Enable K8s GC                            |
| `kubernetes.namespace`      | string | Yes      | Pod namespace                            |
| `kubernetes.name`           | string | No       | Pod name (or use label_selector)         |
| `kubernetes.label_selector` | string | No       | Label selector for pod discovery         |
| `kubernetes.gc_config_path` | string | Yes      | Path to registry config inside container |
| `kubernetes.gc_delete_unreferenced_blobs` | bool | No | Delete unreferenced blobs during GC |
| `kubernetes.delete_empty_repos` | bool | No | Delete empty repository directories via `exec` |
| `kubernetes.storage_path`   | string | No       | Registry storage path (default: `/var/lib/registry`) |

#### Docker

| Field                       | Type   | Required | Description                              |
| --------------------------- | ------ | -------- | ---------------------------------------- |
| `docker.enabled`            | bool   | Yes      | Enable Docker GC                         |
| `docker.container`          | string | Yes      | Container name or ID                     |
| `docker.gc_config_path`     | string | Yes      | Path to registry config inside container |
| `docker.gc_delete_unreferenced_blobs` | bool | No | Delete unreferenced blobs during GC |

### Full Example

```yaml
log_level: info
dry_run: false
concurrency: 2
timeout: 60s

registries:
  - name: production
    url: https://registry.prod.example.com
    username_env: REGISTRY_USER
    password_env: REGISTRY_PASS
    keep: 5
    delete_untagged: true
    kubernetes:
      enabled: true
      namespace: registry
      label_selector: app=docker-registry
      gc_config_path: /etc/docker/registry/config.yml
      gc_delete_unreferenced_blobs: true
    repos:
      api-backend:
        keep: 10
        tag_filter: "^v\\d+\\.\\d+\\.\\d+$"
      frontend:
        skip: true
      worker:
        keep: 3

  - name: staging
    url: http://registry.staging:5000
    keep: 3
    docker:
      enabled: true
      container: registry-staging
      gc_config_path: /etc/docker/registry/config.yml
```

## Deployment

### Kubernetes (Helm)

```sh
helm repo add drc https://eznix86.github.io/docker-registry-cleaner
helm repo update

helm install drc drc/drc -n drc --create-namespace
```

Create a Kubernetes Secret for registry credentials instead of putting passwords in `values.yaml`:

```sh
kubectl create namespace drc

kubectl create secret generic registry-cleaner-credentials \
  -n drc \
  --from-literal=username='YOUR_USERNAME' \
  --from-literal=password='YOUR_PASSWORD'
```

Reference the Secret through environment variables in your Helm values:

```yaml
config:
  log_level: info
  dry_run: true
  concurrency: 1
  timeout: 30s
  registries:
    - name: registry
      url: http://docker-registry.docker-registry.svc.cluster.local:5000
      keep: 5
      username_env: REGISTRY_USERNAME
      password_env: REGISTRY_PASSWORD
      delete_untagged: true
      kubernetes:
        enabled: true
        namespace: docker-registry
        label_selector: app.kubernetes.io/name=docker-registry
        gc_config_path: /etc/docker/registry/config.yml
        gc_delete_unreferenced_blobs: true

env:
  - name: REGISTRY_USERNAME
    valueFrom:
      secretKeyRef:
        name: registry-cleaner-credentials
        key: username
  - name: REGISTRY_PASSWORD
    valueFrom:
      secretKeyRef:
        name: registry-cleaner-credentials
        key: password
```

Then install or upgrade with your values file:

```sh
helm upgrade --install drc drc/drc \
  -n drc \
  --create-namespace \
  -f values.yaml
```

### Docker Compose

```yaml
services:
  drc:
    image: ghcr.io/eznix86/docker-registry-cleaner:latest
    volumes:
      - ./config.yaml:/etc/drc/config.yaml
    command: ["-c", "/etc/drc/config.yaml"]
```

### Docker

```bash
docker run --rm \
  -v $(pwd)/config.yaml:/etc/drc/config.yaml \
  ghcr.io/eznix86/docker-registry-cleaner:latest \
  -c /etc/drc/config.yaml
```

## Garbage Collection

DRC triggers GC by executing `registry garbage-collect` inside the running registry container:

> [!IMPORTANT]
> The registry must be stopped or in read-only mode during GC to avoid data corruption. DRC does not handle this automatically — ensure your deployment strategy accounts for this (e.g., scale to 0 replicas before GC).

## Development

```bash
# Build
task build

# Run locally
task run

# Test against local registries
task test:setup
task test:run
task test:teardown
```

## License

MIT
