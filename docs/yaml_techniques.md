# Appendix: YAML Techniques

Most of this guide has been focused on writing the template language. Here, we'll look at the YAML format. YAML has some useful features that we, as template authors, can use to make our templates less error‑prone and easier to read.

## Scalars and Collections
According to the YAML spec, there are two types of collections, and many scalar types.

**Maps**:
```yaml
map:
  one: 1
  two: 2
  three: 3
```

**Sequences**:
```yaml
sequence:
  - one
  - two
  - three
```

Scalar values are individual values (as opposed to collections).

## Scalar Types in YAML
In Helm's dialect of YAML, the scalar data type of a value is determined by a complex set of rules, including the Kubernetes schema for resource definitions. When inferring types, the following rules tend to hold true.

- Unquoted numbers are treated as numeric types:
  ```yaml
  count: 1          # int
  size: 2.34        # float
  ```
- Quoted numbers become strings:
  ```yaml
  count: "1"       # string, not int
  size: '2.34'      # string, not float
  ```
- Booleans follow the same pattern:
  ```yaml
  isGood: true      # bool
  answer: "true"   # string
  ```
- The word for an empty value is `null` (not `nil`).

> Note: `port: "80"` is valid YAML and will pass through the template engine, but Kubernetes expects an integer and will reject it.

### Forcing Types with YAML Node Tags
```yaml
coffee: "yes, please"
age: !!str 21          # force string
port: !!int "80"        # force integer even when quoted
```

## Strings in YAML
YAML provides several ways to represent strings.

### Inline Styles (single‑line)
```yaml
way1: bare words
way2: "double‑quoted strings"
way3: 'single‑quoted strings'
```
- **Bare words** are unquoted and not escaped.
- **Double‑quoted** strings support back‑slash escapes (e.g., `"Hello\nWorld"`).
- **Single‑quoted** strings are literal; the only escape is `''` for a single quote.

### Multi‑line Strings
```yaml
coffee: |
  Latte
  Cappuccino
  Espresso
```
The `|` preserves line breaks. Indentation matters – incorrect indentation results in a parse error.

#### Protecting the First Line
```yaml
coffee: |
  # Commented first line
  Latte
  Cappuccino
  Espresso
```
The comment line will appear in the rendered value.

### Controlling Trailing Newlines
- `|‑` removes the trailing newline.
- `|+` preserves all trailing whitespace.

### Folded Multi‑line Strings
Use `>` to fold lines into a space‑separated string.
```yaml
coffee: >
  Latte
  Cappuccino
  Espresso
```
Result: `Latte Cappuccino Espresso\n`

### Embedding Multiple Documents
```yaml
---
 document: 1
...
---
 document: 2
```
Multiple YAML documents can be placed in one file, separated by `---`. Helm templates may contain multiple documents, but most Helm files (e.g., `values.yaml`) should contain only one.

## YAML is a Superset of JSON
Any valid JSON is valid YAML.
```json
{ "coffee": "yes, please", "coffees": ["Latte", "Cappuccino", "Espresso"] }
```
Can be written as:
```yaml
coffee: yes, please
coffees:
  - Latte
  - Cappuccino
  - Espresso
```
Mixed styles are allowed with care.

## YAML Anchors
Anchors let you reuse values.
```yaml
coffee: "yes, please"
favorite: &favoriteCoffee "Cappuccino"
coffees:
  - Latte
  - *favoriteCoffee
  - Espresso
```
During the first parse the anchor is expanded; subsequent renders lose the anchor.

---
*This appendix is part of the Helmify documentation hub.*
