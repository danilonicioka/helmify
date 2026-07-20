# Helm Templates Reference Guide

This guide provides a comprehensive reference on the Helm template engine, Go data types, built-in objects, functions, control structures, and naming conventions.

---

## 1. Introduction to Helm Templates

Helm templates are written in the Go template language (`text/template`), enhanced with 50+ helper functions from the Sprig library and Kubernetes-specific functions. When a chart is installed or upgraded, Helm runs the template engine over all files inside `templates/` and sends the rendered manifests to the Kubernetes API server.

---

## 2. Built-in Objects

Every template has access to a set of pre-defined, read-only objects. All built-in objects start with an uppercase letter:

- **`.Values`**: Configurable values passed into the template from `values.yaml` or command-line overrides (`--set`).
- **`.Release`**: Information about the release runtime:
  - `.Release.Name`: The release name.
  - `.Release.Namespace`: Target namespace.
  - `.Release.IsInstall` / `.Release.IsUpgrade`: Boolean flags indicating the current operation.
  - `.Release.Service`: The service rendering the templates (defaults to `Helm`).
- **`.Chart`**: Metadata from `Chart.yaml` (e.g. `.Chart.Name`, `.Chart.Version`, `.Chart.AppVersion`).
- **`.Files`**: A map of non-special files in the chart (excluding `templates/` or files in `.helmignore`). Access file contents using `.Files.Get` or `.Files.GetBytes`.
- **`.Capabilities`**: Cluster capability queries:
  - `.Capabilities.KubeVersion`: Target Kubernetes version.
  - `.Capabilities.APIVersions.Has "apps/v1"`: Checks if an API group/version is supported.

---

## 3. Go Data Types and Templates

Because Go is a strongly typed language, values in Helm templates carry concrete types:
- `string`: `"text"`
- `bool`: `true` / `false`
- `int` / `float64`: Numeric types
- `[]byte`: Slice of bytes
- `struct`: Objects with named fields and methods
- `slice`: Indexed arrays (`[]string`, `[]interface{}`)
- `map[string]interface{}`: String-keyed dictionary maps

### Type Introspection
To inspect the type of an object during debugging, print it using:
```yaml
typeOfValue: {{ printf "%T" .Values.someValue }}
# Or use the helper functions:
kind: {{ kindOf .Values.someValue }}
type: {{ typeOf .Values.someValue }}
```

---

## 4. Variables and Scoping

Variables allow you to capture state and reference values across block scopes (e.g. inside a `with` or `range` block). Variables are prefixed with `$` and assigned using `:=`.

```yaml
{{- $releaseName := .Release.Name -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ $releaseName }}-config
```

### Scope Resolution
Inside a `with` or `range` block, the dot (`.`) context is re-scoped to the inner object. To reference the global context, use the `$` variable (e.g. `$.Release.Name`).

---

## 5. Pipelines and Functions

Pipelines chain multiple template directives together using the pipe character (`|`). The output of the left side is passed as the final argument to the function on the right side.

```yaml
# Replaces "+" with "_" and truncates to 63 chars
name: {{ .Chart.Name | replace "+" "_" | trunc 63 }}
```

### Common Sprig Functions
- **String Manipulation**: `lower`, `upper`, `title`, `trim`, `replace`, `trunc`, `trimSuffix`.
- **Defaulting**: `default "fallback-value" .Values.customValue`.
- **Serialization**: `toYaml`, `toJson`.

---

## 6. Logic and Flow Control

### Conditional Blocks (`if`/`else`)
```yaml
{{- if .Values.enabled }}
status: active
{{- else if .Values.pending }}
status: pending
{{- else }}
status: disabled
{{- end }}
```

### Context Scoping (`with`)
Modifies the current scope (`.`) to point to a nested key:
```yaml
{{- with .Values.image }}
image: "{{ .repository }}:{{ .tag }}"
{{- end }}
```

### Loops (`range`)
Iterates over lists or maps:
```yaml
ports:
  {{- range .Values.ports }}
  - containerPort: {{ . }}
  {{- end }}
```

---

## 7. Named Templates and Helpers

Named templates (also called partials or helper templates) allow you to declare reusable blocks. They are declared using `define` and included using `template` or `include`.

> **Note**: Template names are global. Prefix helper names with the chart name to prevent collisions.

```yaml
{{- define "mychart.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

### `template` vs. `include`
- **`template`**: A native Go template action. Cannot pass output to other functions (e.g. you cannot pipe it to `nindent`).
- **`include`**: A custom Helm function. Loads the helper output as a string, allowing you to pipe and manipulate it:
  ```yaml
  labels:
    {{- include "mychart.labels" . | nindent 4 }}
  ```

---

## 8. Template Debugging Techniques

1. **`helm lint`**: Checks that your templates conform to Helm structural best practices and YAML formatting.
2. **`helm template --debug`**: Renders all templates locally and prints the output. Displays line numbers and rendering details if parser errors occur.
3. **`helm install --dry-run --debug`**: Renders templates locally and validates them against the target Kubernetes cluster API server without executing changes.
4. **Commenting Out**: If a block fails to render, comment it out using template comments (`{{/* comment */}}`) to isolate the issue.
