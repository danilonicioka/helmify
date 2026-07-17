# Named Templates

Helm allows you to split your chart templates into reusable, named fragments (partials). These fragments can be defined once and referenced multiple times throughout your chart, improving maintainability and reducing duplication.

## Declaring a Named Template
Use the `define` action to create a template with a global name:

```gotemplate
{{- define "mychart.labels" }}
  labels:
    generator: helm
    date: {{ now | htmlDate }}
{{- end }}
```

- The name is global across all templates in the chart (including subcharts).
- By convention, prefix the name with the chart name to avoid conflicts (`mychart.labels`).
- Place definitions in a file that starts with an underscore (e.g., `_helpers.tpl`) so Helm knows it should not render a manifest directly.

## Using a Named Template
### `template`
The `template` action inserts the rendered content of a named template directly into the surrounding template. You must pass a scope (`.`) if the defined template needs access to chart objects.

```gotemplate
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  {{- template "mychart.labels" . }}
```

If you omit the scope (`.`), the defined template sees only the default (empty) scope, so references like `.Chart.Name` will be `nil`.

### `include`
`include` works like `template` but returns the rendered content as a string, allowing further pipeline processing (e.g., indentation with `nindent`). This is the preferred approach for YAML because it lets you control whitespace.

```gotemplate
metadata:
  labels:
    {{- include "mychart.labels" . | nindent 4 }}
```

## File Naming Convention
- Files under `templates/` that **do not** start with `_` are rendered as Kubernetes manifests.
- Files that **do** start with `_` (e.g., `_helpers.tpl`) are treated as helper/partial files and are **not** rendered directly.
- Keep all your `define` blocks in such helper files.

## Passing Scope
When you call a named template, the context (`.`) you pass determines what objects are available inside the defined template.

```gotemplate
{{- template "mychart.labels" . }}        # full top‑level scope
{{- template "mychart.labels" .Values }}   # only values are visible
{{- include "mychart.labels" $ }}         # `$` always points to the root context
```

## Versions and Naming Conflicts
- Since names are global, two templates with the same name will cause the later one to win.
- To avoid clashes across chart versions, embed the version in the name:
  - `{{ define "mychart.v1.labels" }}`
  - `{{ define "mychart.v2.labels" }}`

## Example: Labels Helper
```gotemplate
{{/* Generate basic labels */}}
{{- define "mychart.labels" }}
labels:
  generator: helm
  date: {{ now | htmlDate }}
  chart: {{ .Chart.Name }}
  version: {{ .Chart.Version }}
{{- end }}
```

Usage in a manifest:

```gotemplate
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
  {{- include "mychart.labels" . | nindent 2 }}
```

Produces correctly indented YAML with chart name and version.

## Summary
- **Define** (`{{ define "name" }}`) creates a reusable snippet.
- **Template** (`{{ template "name" . }}`) injects the snippet directly.
- **Include** (`{{ include "name" . }}`) returns the snippet as a string for further processing (recommended for YAML).
- Store definitions in `_*.tpl` files, use chart‑specific prefixes, and pass the appropriate scope.

For deeper details on whitespace handling and other template features, see the **Flow Control** documentation.
