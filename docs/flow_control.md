# Flow Control

Control structures (called "actions" in template parlance) provide you, the template author, with the ability to control the flow of a template's generation. Helm's template language provides the following control structures:

- `if`/`else` for creating conditional blocks
- `with` to specify a scope
- `range`, which provides a "for each"-style loop

In addition to these, it provides a few actions for declaring and using named template segments:

- `define` declares a new named template inside of your template
- `template` imports a named template
- `block` declares a special kind of fillable template area

In this section, we'll talk about `if`, `with`, and `range`. The others are covered in the "Named Templates" section later in this guide.

## If/Else
The first control structure we'll look at is for conditionally including blocks of text in a template. This is the `if`/`else` block.

The basic structure for a conditional looks like this:

```gotemplate
{{ if PIPELINE }}
  # Do something
{{ else if OTHER PIPELINE }}
  # Do something else
{{ else }}
  # Default case
{{ end }}
```

A pipeline is evaluated as false if the value is:

- a boolean `false`
- a numeric zero
- an empty string
- `nil` (empty or null)
- an empty collection (map, slice, tuple, dict, array)

Under all other conditions, the condition is true.

### Example
Add a simple conditional to a `ConfigMap` to include a `mug` flag when the drink is set to `coffee`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}
  {{ if eq .Values.favorite.drink "coffee" }}mug: "true"{{ end }}
```

If `drink` is not `coffee`, the `mug` line is omitted.

## Controlling Whitespace
Whitespace handling is crucial because Helm leaves whitespace outside of `{{` and `}}` untouched, which can break YAML indentation. Helm offers chomping modifiers:

- `{{-` trims whitespace to the left
- `-}}` trims whitespace to the right

Example with proper whitespace control:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}
  {{- if eq .Values.favorite.drink "coffee" }}
  mug: "true"
  {{- end }}
```

Be careful not to over‑chomp, which can collapse newlines and produce `food: "PIZZA"mug: "true"`.

## Modifying Scope Using `with`
The `with` action sets a new scope (`.`) for the block, allowing shorter references:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  {{- end }}
```

Inside the `with` block, `.drink` and `.food` refer to the fields under `.Values.favorite`. Use `$` to access the root scope (`{{ $.Release.Name }}`) if needed.

## Looping with `range`
`range` iterates over collections (lists, maps, tuples). Example with a list of pizza toppings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  {{- end }}
  toppings: |-
    {{- range .Values.pizzaToppings }}
    - {{ . | title | quote }}
    {{- end }}
```

`range` also works with maps and tuples. For a tuple example:

```gotemplate
{{- range tuple "small" "medium" "large" }}
- {{ . }}
{{- end }}
```

### Whitespace with `range`
Use `{{-` and `-}}` to control newlines inside loops, preventing extra blank lines.

---

For more details on whitespace control, see the official Go template documentation.
