# Helm, Kustomize & OpenShift Guide

This document collects essential concepts, best‑practice recommendations, and quick‑reference commands for working with **Helm**, **Kustomize**, and **OpenShift** in the context of Helmify.

## Helm
- **Version compatibility** – Helmify requires Helm ≥ 3.6.0. Use the same version locally as in CI to avoid template mismatches.
- **Chart structure** – Follow the standard layout:
  ```
  Chart.yaml
  values.yaml
  templates/
  charts/   # optional sub‑charts
  ```
- **Non‑root containers** – Helmify enforces non‑root pods. In your `values.yaml` you can set:
  ```yaml
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    capabilities:
      drop: ["ALL"]
  ```
- **SCC (Security Context Constraints) for OpenShift** – When deploying to OpenShift, ensure the generated pod spec includes the required SCC annotations (e.g., `securityContext: {fsGroup: 1000}`) and that the target project has the appropriate SCC bound.
- **Image Pull Secrets** – Helmify can inject existing image‑pull‑secrets via the `-image-pull-secrets` flag. The secret must exist in the target namespace.

## Kustomize
- **Build command** – `kustomize build <dir>` produces a single concatenated YAML stream. Feed this directly to Helmify:
  ```bash
  kustomize build ./kustomize | helmify mychart
  ```
- **Common pitfalls**:
  - **Name suffix collisions** – Kustomize adds a hash suffix (10‑character). Helmify strips these automatically, but custom suffixes of exactly 10 characters (e.g., `-judiciaria`) are also stripped. Whitelist such suffixes in `StripKustomizeHash` if you need to keep them.
  - **Resource ordering** – Ensure `kustomization.yaml` lists resources in the order you expect; Helmify processes them sequentially.
- **Integration tip** – Keep a `kustomization.yaml` alongside the generated Helm chart for easy diffing between raw manifests and templated output.

## OpenShift Specifics
- **Routes** – Helmify automatically maps OpenShift `Route` objects to the correct component values. Verify that `spec.to.name` points to an existing `Service`.
- **SCC & Non‑Root** – OpenShift restricts containers from running as root unless the `anyuid` SCC is explicitly granted. Prefer non‑root images from Red Hat registry (`registry.access.redhat.com`).
- **Image Registry** – Use the corporate registry `tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br`. If you need an image from Docker Hub, mirror it into the private registry and reference the mirrored tag.
- **Deploying Helmify** – The Helmify service runs as a container on OpenShift. Ensure the container image is built from the upstream repo (see rule 9) and the pod security context complies with the cluster’s SCC.

## Quick Commands
```bash
# Validate a Helm chart locally (requires helm binary)
helm lint ./mychart

# Run Helmify inside a container (recommended for CI)
docker run --rm -v $(pwd):/work -w /work ghcr.io/redhat/ubi8 go run ./cmd/helmify -f ./manifests mychart

# Mirror a Docker Hub image to the private registry
skopeo copy docker://library/nginx:latest docker://tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/nginx:latest
```
