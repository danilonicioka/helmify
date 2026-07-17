# Accessing Files Inside Templates

Helm can embed the raw contents of files from the chart package using the **`.Files`** object. This lets you add configuration data, scripts, or any other static files directly into Kubernetes manifests without processing them through the Go template engine.

---
## Important Caveats
- Files must be **inside the chart directory** and **not** in `templates/`.
- Files excluded via **`.helmignore`** are unavailable.
- Chart size is limited to **~1 MiB** (Kubernetes object size limits).
- Permissions and UNIX mode bits are ignored – Helm packages files as plain data.

---
## Basic Example
Assume three TOML files at the chart root:

```text
config1.toml
message = "Hello from config 1"

config2.toml
message = "This is config 2"

config3.toml
message = "Goodbye from config 3"
```

You can render them into a `ConfigMap` with a single template:

```gotemplate
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  {{- $files := .Files }}
  {{- range tuple "config1.toml" "config2.toml" "config3.toml" }}
  {{ . }}: |-
    {{ $files.Get . }}
  {{- end }}
```

Resulting YAML:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: quieting-giraf-configmap
data:
  config1.toml: |-
    message = "Hello from config 1"

  config2.toml: |-
    message = "This is config 2"

  config3.toml: |-
    message = "Goodbye from config 3"
```

---
## Path Helpers (Go `path` package)
Helm mirrors Go's `path` functions (lower‑cased) for manipulating file paths:

- `base` – basename of a path
- `dir` – directory component
- `ext` – file extension
- `isAbs` – absolute‑path check
- `clean` – clean up `..` and `.` segments

These are handy when you need to transform file names before inserting them.

---
## Glob Patterns
`Files.Glob(pattern)` returns a `Files` collection matching the glob. You can then use any `Files` method on the result.

```gotemplate
{{- range $path, $_ := .Files.Glob "**/*.yaml" }}
{{ $.Files.Get $path }}
{{- end }}
```

---
## ConfigMap & Secret Helpers
Helm provides convenience methods to bulk‑populate `ConfigMap` and `Secret` data from a set of files:

```gotemplate
# ConfigMap from all files in "foo/"
{{- (.Files.Glob "foo/*").AsConfig | nindent 2 }}

# Secret (base64‑encoded) from all files in "bar/"
{{- (.Files.Glob "bar/*").AsSecrets | nindent 2 }}
```

---
## Encoding Files
Often you need to store binary data in a `Secret`. Use the `b64enc` pipeline on the file contents:

```gotemplate
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-secret
type: Opaque
data:
  token: |-
    {{ .Files.Get "config1.toml" | b64enc }}
```

---
## Accessing Lines
`Files.Lines "path"` returns a slice of the file’s lines, allowing per‑line processing:

```gotemplate
data:
  {{- range .Files.Lines "scripts/startup.sh" }}
  {{ . }}
  {{- end }}
```

---
## Scope & Root Context
When using `.Files` inside a named template, remember to pass the correct scope (usually `.` or `$`). The `$` variable always points to the root chart context, ensuring you can reach `.Files` regardless of the current block.

---
## Summary
- **`.Files`** gives direct access to chart‑packaged files.
- Use **`Get`**, **`Glob`**, **`AsConfig`**, **`AsSecrets`**, **`Lines`**, and **encoding pipelines** for flexible inclusion.
- Respect chart size limits and `.helmignore` exclusions.
- Combine with **named templates** and **scope variables** for clean, reusable manifests.

For more on whitespace handling, see the **Flow Control** documentation.
