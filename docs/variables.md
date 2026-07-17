# Variables

In Helm templates, variables provide a way to store and reuse values, making complex templates easier to read and maintain. Variables are named using the `$` prefix (e.g., `$name`) and are assigned with the `:=` operator.

## Declaring Variables
```gotemplate
{{- $relname := .Release.Name -}}
```
The above assigns the release name to `$relname`. Once declared, the variable is accessible within the current scope and any nested scopes.

## Scoping Rules
- Variables are **scoped to the block** in which they are declared.
- A variable declared at the top level of a template is available throughout the entire file.
- Variables declared inside a `with` or `range` block are only available within that block.
- The special variable `$` always points to the **root context** (the top‑level `.`, which includes `.Release`, `.Chart`, `.Values`, etc.).

## Using Variables with `with`
When you need access to objects that are outside the current `with` scope, assign them to a variable before entering the block:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- $relname := .Release.Name -}}
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  release: {{ $relname }}
  {{- end }}
```

## Variables in `range` Loops
`range` can expose both the **index** (or key) and the **value** of a collection. The syntax is:

```gotemplate
{{- range $index, $value := .Values.pizzaToppings }}
  {{ $index }}: {{ $value }}
{{- end }}
```
Resulting output:

```yaml
  toppings: |-
    0: mushrooms
    1: cheese
    2: peppers
    3: onions
```

When iterating over a map (or object), you receive the key and the value:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- range $key, $val := .Values.favorite }}
  {{ $key }}: {{ $val | quote }}
  {{- end }}
```
Produces:

```yaml
  drink: "coffee"
  food: "pizza"
```

## Accessing the Root Context (`$`)
Inside deeply nested blocks, you may need to reference top‑level values such as the chart name or release name. Use the `$` variable, which never changes:

```yaml
{{- range .Values.tlsSecrets }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .name }}
  labels:
    app.kubernetes.io/name: {{ template "fullname" $ }}
    helm.sh/chart: "{{ $.Chart.Name }}-{{ $.Chart.Version }}"
    app.kubernetes.io/instance: "{{ $.Release.Name }}"
    app.kubernetes.io/version: "{{ $.Chart.AppVersion }}"
    app.kubernetes.io/managed-by: "{{ $.Release.Service }}"
type: kubernetes.io/tls
data:
  tls.crt: {{ .certificate }}
  tls.key: {{ .key }}
{{- end }}
```

## Summary
- Declare variables with `{{- $var := value -}}`.
- Variables respect block scope; top‑level variables are globally available within the template.
- Use `$` to always reference the root context.
- Combine variables with `with` and `range` to simplify complex templates and avoid repetitive long paths.

For more details on whitespace handling and other template features, see the **Flow Control** documentation.
