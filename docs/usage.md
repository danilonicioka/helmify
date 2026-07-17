# Usage

## CLI

```bash
# Generate a chart from a single manifest file
cat my-app.yaml | helmify mychart

# Generate from a directory (recursive)
helmify -f ./manifests -r mychart

# Use with Kustomize output
kustomize build ./kustomize-dir | helmify mychart
```

## API

```bash
curl -X POST \
  -H "X-Chart-Name: my-chart" \
  --data-binary @my-app.yaml \
  http://<helmify-api-url>/v1/generate \
  --output my-chart.tar.gz
```

### Common Flags

| Flag | Description |
|------|-------------|
| `-f` | Input file or directory |
| `-r` | Recursively scan directories |
| `-v` | Verbose output |
| `-vv`| Very verbose (debug) |
| `-original-name` | Preserve original resource names |
| `-preserve-ns` | Keep original namespaces |
| `-image-pull-secrets` | Use existing image pull secrets |
| `-cert-manager-as-subchart` | Install cert‑manager as a sub‑chart |

Refer to the full flag list in the [README](../README.md#available-options).
