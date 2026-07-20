# Getting Started and Usage Guide

This guide provides an overview of Helmify, instructions for installing and running the tool via the CLI or Web API, and a tutorial to get you started with creating your first Helm chart.

---

## 1. Overview

Helmify is a CLI and Web API service that generates **Helm charts** from Kubernetes manifests. It supports plain manifests, Kustomize output, and OpenShift-specific resources (Routes, SCC, etc.). The tool aims to provide production‑ready, TJPA‑compliant charts with consistent labeling, non‑root containers, and deterministic rollouts.

### Key Benefits
- **Simplifies Migration**: Transition easily from raw YAML manifests or Kustomize setups to reusable Helm charts.
- **Enforces Standards**: Automatically injects standard labels, annotations, probes, configurations, and SCC compliance tags.
- **Microservices Ready**: Can be run locally as a CLI tool or deployed to OpenShift as a web-based conversion service.

---

## 2. Usage

### CLI Usage
Generate charts by piping manifests to `helmify` or specifying files/directories:

```bash
# Generate a chart from a single manifest file
cat my-app.yaml | helmify mychart

# Generate from a directory (recursive)
helmify -f ./manifests -r mychart

# Use with Kustomize output
kustomize build ./kustomize-dir | helmify mychart
```

### Common CLI Flags
- `-f`: Input file or directory path.
- `-r`: Recursively scan directories for YAML manifests.
- `-v` / `-vv`: Set verbose or debug logging output.
- `-original-name`: Preserves original resource names instead of templating them with the release fullname.
- `-preserve-ns`: Keeps original namespaces inside the manifests.
- `-image-pull-secrets`: Uses existing image pull secrets configurations.

### Web REST API Usage
Send a raw manifest payload to the endpoint to download a generated chart archive:

```bash
curl -X POST \
  -H "X-Chart-Name: my-chart" \
  --data-binary @my-app.yaml \
  http://<helmify-api-url>/v1/generate \
  --output my-chart.tar.gz
```

---

## 3. Creating Your First Chart

To understand how templates are structured, let's create a chart named `mychart` from scratch.

### Create the Chart scaffolding
Use the Helm CLI to generate a baseline directory structure:
```bash
$ helm create mychart
Creating mychart
```

The resulting `mychart` directory contains:
- `Chart.yaml`: Chart metadata.
- `values.yaml`: Default configuration values.
- `templates/`: Manifest files (e.g. `deployment.yaml`, `service.yaml`, `_helpers.tpl`).

For this tutorial, let's clean the templates directory to start fresh:
```bash
$ rm -rf mychart/templates/*
```

### Add a ConfigMap Template
Create `mychart/templates/configmap.yaml`:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
```

The dot (`.`) represents the current scope context, giving access to the built-in `Release` variables.

### Dry-run and Validate
Test the template rendering locally without installing it to a cluster:
```bash
$ helm install --debug --dry-run test-release ./mychart
```
Verify the output renders `test-release-configmap` as the ConfigMap name.
