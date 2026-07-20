# Charts and Values Guide

This guide details the physical layout of Helm charts, the structure and merge mechanics of `values.yaml`, subcharts, dependencies, local file access, and advanced YAML syntax patterns.

---

## 1. Chart File Structure

A Helm chart is packaged as a versioned directory matching the chart name:

```
mychart/
  Chart.yaml          # Metadata definition
  LICENSE             # Optional plain text license
  README.md           # User documentation
  values.yaml         # Default configuration values
  values.schema.json  # Optional JSON schema validation
  charts/             # Dependency charts directory
  crds/               # Plain Custom Resource Definitions
  templates/          # Go templates directory
```

---

## 2. Values and Validation

Values customize a template rendering. Helm merges configuration sources in this order (highest precedence wins):
1. Inline overrides via `--set key=value`
2. Values files passed via CLI flag `-f`/`--values`
3. Default `values.yaml` inside the chart package

### Values Validation via JSON Schema
An optional `values.schema.json` validates values against schema definitions during `helm install`, `upgrade`, `lint`, and `template`.
Example schema block:
```json
{
  "$schema": "https://json-schema.org/draft-07/schema#",
  "properties": {
    "replicas": {
      "type": "integer",
      "minimum": 1
    }
  },
  "required": ["replicas"]
}
```

---

## 3. Subcharts, Dependencies & Globals

Charts can depend on other charts (subcharts).

### Dependency Management
Declare dependencies in `Chart.yaml`:
```yaml
dependencies:
  - name: mariadb
    version: 11.x.x
    repository: https://charts.bitnami.com/bitnami
    condition: mariadb.enabled
```
Run `helm dependency update` to pull the tarball archives into the `charts/` folder.

### Dependency Customization
- **Alias**: Renames a dependency so it can be included multiple times with separate configurations.
- **Condition**: YAML path mapping to a boolean in parent values to enable/disable the subchart.
- **Tags**: Groups subcharts to enable or disable them collectively.
- **import-values**: Explicitly copies child exports to the parent values context.

### Global Values
`global` keys in `values.yaml` are passed downward to all subcharts and are accessible via `.Values.global`.

---

## 4. Accessing Files Inside Templates

Templates can import non-templated static files from the chart directory using the `.Files` object.

### Access Patterns
- **Get file content as string**: `{{ .Files.Get "config.txt" }}`
- **Get file content as byte array**: `{{ .Files.GetBytes "binary.dat" }}`
- **Glob pattern matching**:
  ```yaml
  {{- range $path, $_ :=  .Files.Glob "config/**" }}
  {{ $path }}: {{ $.Files.Get $path | b64enc }}
  {{- end }}
  ```

### Limitations and `.helmignore`
- Files inside `templates/` cannot be accessed via `.Files`.
- Files matched by patterns in `.helmignore` at the chart root are excluded from the package and cannot be read.

---

## 5. YAML Techniques for Helm Templates

### Scalars and Types
Barewords identify numbers/booleans:
```yaml
count: 80         # integer
isProduction: true # boolean
```
Quoted values force string evaluation:
```yaml
port: "80"        # string
```

### Multiline Strings
- **Literal (`|`)**: Preserves all newlines.
- **Chomping (`|-`)**: Strips trailing newlines.
- **Preserve (`|+`)**: Retains all trailing newlines and whitespace.
- **Folded (`>`)**: Collapses internal newlines into spaces.

### YAML Anchors
Anchors (`&`) and aliases (`*`) allow referencing and copying mapping structures:
```yaml
defaultConfig: &defaults
  port: 8080
  protocol: TCP

serviceBackend:
  <<: *defaults
```
> **Warning**: Helm or Kubernetes parsers may expand anchors and discard them upon saving, removing them from rewritten manifests.

## Route Manifests

Helmify generates **individual route manifests** for each route type (default, internal, external) as separate files:

- `templates/route-default.yaml`
- `templates/route-int.yaml`
- `templates/route-ext.yaml`

These files contain a **single `Route` resource** each, making it easy to manage, review, or apply them individually (e.g., via `kubectl apply -f route-default.yaml`).

In this chart, **only the individual route manifests** (`templates/route-default.yaml`, `templates/route-int.yaml`, `templates/route-ext.yaml`) are generated. There is no combined route file, simplifying the chart structure and making each route explicit.


### Why both?
- **Granular files** give developers clear, isolated resources for each route, simplifying debugging and version control.
- **Combined file** provides a convenient one‑stop‑shop for users who prefer a single `helm template` output, especially when generating CI/CD artifacts.

Both approaches are fully supported by Helm; you can choose the one that fits your workflow. The documentation now reflects this dual‑manifest strategy.

## values‑ca.yaml (Developer‑friendly Values)

`values‑ca.yaml` is an **automatically generated** file that contains only the configuration elements relevant for developers when working locally:

- The **full `global` block** (including any scalar entries such as `TZ` and the `cm`/`secret` sub‑maps) is preserved, so developers can see and override cluster‑wide settings.
- For each component deployment, only the **`cm` (ConfigMap) and `secret`** sections are kept. All infrastructure‑only fields like `replicas`, `image`, `repository`, `tag`, `port`, etc., are omitted because they are managed by CI/CD pipelines and should not be edited directly by developers.
- The resulting file is intended to be **checked‑in** or **mounted** into a local development container to simplify configuration management without exposing internal deployment details.

### Example
```yaml
global:
  TZ: America/Sao_Paulo
  cm:
    LOG_LEVEL: debug
  secret:
    DATABASE_PASSWORD: "s3cr3t"

my‑app:
  cm:
    VAR_A: VAL_A
  secret:
    SECRET_B: VAL_B
```

In the example above, the `global` block retains the `TZ` scalar and the `cm`/`secret` maps, while the component `my‑app` only includes its `cm` and `secret` entries. Fields such as `replicas`, `repository`, `tag`, and `port` are intentionally omitted.

Developers can modify this file to adjust runtime configuration, and the Helm chart generation process will merge it with the full `values.yaml` during deployment.
