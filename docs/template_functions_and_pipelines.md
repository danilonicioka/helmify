# Template Functions and Pipelines

> **Warning**: This page has not yet been updated for Helm 4. Some of the content might be inaccurate or not applicable to Helm 4. For more information about the Helm 4 new features, improvements, and breaking changes, see **Helm 4 Overview**.

This section explains how to transform data inside Helm templates using functions and pipelines.

## Quoting Values

When injecting strings from `.Values` into a manifest, quote them to ensure they are valid YAML strings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap

data:
  myvalue: "Hello World"
  drink: {{ quote .Values.favorite.drink }}
  food: {{ quote .Values.favorite.food }}
```

## Pipelines

A pipeline sends the output of the left‑hand expression as the *last* argument to the function on the right, similar to UNIX pipes.

```gotemplate
drink: {{ .Values.favorite.drink | quote }}
food:  {{ .Values.favorite.food  | upper | quote }}
```

You can chain multiple functions:

```gotemplate
drink: {{ .Values.favorite.drink | repeat 5 | quote }}
```

Resulting snippet:

```yaml
drink: "coffeecoffeecoffeecoffeecoffee"
```

## The `default` Function

Provide a fallback when a value is missing:

```gotemplate
drink: {{ .Values.favorite.drink | default "tea" | quote }}
```

If `favorite.drink` is unset, the rendered ConfigMap will contain `"tea"`.

## Computed Defaults

For values that cannot be stored in `values.yaml`, compute them with `default` and other functions:

```gotemplate
drink: {{ .Values.favorite.drink | default (printf "%s-tea" (include "fullname" .)) }}
```

## The `lookup` Function

Query live cluster resources from a template (requires a real cluster connection, not a dry‑run).

```gotemplate
{{- $ns := lookup "v1" "Namespace" "" "mynamespace" -}}
{{- $annotations := $ns.metadata.annotations -}}
```

When looking up a list, iterate over `.items`:

```gotemplate
{{- range $i, $svc := (lookup "v1" "Service" "mynamespace" "").items -}}
  {{ $svc.metadata.name }}
{{- end -}}
```

## Operators as Functions

Logical and comparison operators are functions and can be used inside pipelines or parentheses:

```gotemplate
{{ if and (eq .Values.env "prod") (gt .Values.replicas 1) }}
  # production specific logic
{{ end }}
```

## Checklist
- [ ] Quote string values from `.Values` using `quote`.
- [ ] Prefer pipeline syntax (`|`) over function‑first syntax.
- [ ] Use `default` for optional values.
- [ ] Remember `lookup` requires a live cluster; avoid during `--dry-run`.
- [ ] Use operators (`eq`, `ne`, `gt`, `and`, `or`, …) as functions for conditionals.

These practices enable clean, readable, and powerful Helm templates.
