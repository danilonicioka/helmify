# Subcharts and Global Values

Helm charts can depend on other charts, called **subcharts**. Subcharts are packaged as independent charts inside the parent chart’s `charts/` directory. Understanding how values are propagated and how to share templates across chart boundaries is essential for building modular, reusable Helm packages.

---
## Subcharts Basics
- A **subchart** is a self‑contained chart located under `mychart/charts/`.
- Subcharts **cannot** directly reference parent‑chart values; they see only their own `.Values`.
- The parent chart can **override** values for a subchart by providing a nested map under the subchart’s name in the parent’s `values.yaml`.

```yaml
# parent chart values.yaml
mysubchart:
  dessert: ice cream   # overrides mysubchart's default
```

When Helm renders templates, it scopes each subchart’s `{{ .Values }}` to the sub‑map that belongs to that subchart.

---
## Creating a Subchart Example
```bash
cd mychart/charts
helm create mysubchart
rm -rf mysubchart/templates/*   # start from a blank slate
```

### Subchart `values.yaml`
```yaml
# mychart/charts/mysubchart/values.yaml
dessert: cake
```

### Subchart Template (`templates/configmap.yaml`)
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-cfgmap2
data:
  dessert: {{ .Values.dessert }}
```

Running the subchart alone:
```bash
helm install --generate-name --dry-run --debug mychart/charts/mysubchart
```
Produces a ConfigMap with `dessert: cake`.

---
## Overriding Subchart Values from the Parent
Add the following to the **parent** `mychart/values.yaml`:

```yaml
mysubchart:
  dessert: ice cream   # parent‑level override
```

Now a dry‑run of the **parent** chart renders:

```yaml
# mychart/templates/configmap.yaml (parent)
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
... # other data
```

```yaml
# mychart/charts/mysubchart/templates/configmap.yaml (subchart)
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-cfgmap2
data:
  dessert: ice cream   # overridden value
```

Notice that the subchart template **still uses** `{{ .Values.dessert }}` – no need to prefix it with `mysubchart` because Helm automatically scopes the values for that chart.

---
## Global Values
Sometimes a value must be visible to **all** charts (parent and any subcharts). Helm provides a dedicated `global` namespace for this purpose.

### Defining Globals (parent `values.yaml`)
```yaml
global:
  salad: caesar
```

### Accessing Globals
- In any template (parent or subchart) you can read the value with:
  ```gotemplate
  {{ .Values.global.salad }}
  ```
- Globals are **not** automatically merged into each chart’s `.Values`; they are accessed through the dedicated `global` key.

### Example
Parent `templates/configmap.yaml`:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  salad: {{ .Values.global.salad }}
```

Subchart `templates/configmap.yaml`:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-cfgmap2
data:
  dessert: {{ .Values.dessert }}
  salad: {{ .Values.global.salad }}
```
Both ConfigMaps will contain `salad: caesar` after a dry‑run install.

---
## Sharing Templates Between Parent and Subcharts
Because all defined templates are **global**, a helper defined in the parent can be used by a subchart (and vice‑versa). Place shared helpers in a file that starts with an underscore (e.g., `_helpers.tpl`).

```gotemplate
{{/* Shared label helper */}}
{{- define "mychart.labels" }}
labels:
  app: {{ .Chart.Name }}
  version: {{ .Chart.Version }}
{{- end }}
```

Both the parent and subchart can include it:
```gotemplate
{{- include "mychart.labels" . | nindent 2 }}
```

---
## Avoid Using `block`
Helm also supports the `block` keyword from Go templates, but it is **not recommended** for Helm charts because when multiple charts provide the same block name the selection order becomes unpredictable. Prefer `include` (or `template` when you don’t need further processing) for reusable snippets.

---
## Summary
- **Subcharts** are isolated; they see only their own values.
- Parents **override** subchart values via a nested map.
- **Global values** (`.Values.global`) provide a shared namespace across all charts.
- Define reusable helpers in `_helpers.tpl` and use `include` for safe, indented insertion.
- **Avoid** `block`; use `include` for predictable template composition.

For more detail on scope handling and variable usage, see the **Variables** and **Named Templates** sections.
