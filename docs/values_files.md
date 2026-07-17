# Values Files

> **Warning**: This page has not yet been updated for Helm 4. Some of the content might be inaccurate or not applicable to Helm 4. For more information about the Helm 4 new features, improvements, and breaking changes, see **Helm 4 Overview**.

In the previous section we examined the built‑in objects that Helm templates provide. One of those objects is **Values**, which gives access to the values passed into the chart. The values originate from several sources, applied in order of increasing specificity:

1. `values.yaml` in the chart (default values).
2. If this is a sub‑chart, the parent chart’s `values.yaml`.
3. A user‑supplied values file passed with `-f`/`--values`.
4. Individual parameters supplied with `--set` (or `--set-string`).

The later source overrides earlier ones. All sources are plain YAML files, and Helm merges them into the single `.Values` object.

## Simple Example

Add a single key to `mychart/values.yaml`:

```yaml
favoriteDrink: coffee
```

Then reference it in a template, e.g. `mychart/templates/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap

data:
  myvalue: "Hello World"
  drink: {{ .Values.favoriteDrink }}
```

Running a dry‑run renders the value from the default file:

```bash
helm install geared-marsupi ./mychart --dry-run=client --debug
```

Result snippet:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: geared-marsupi-configmap

data:
  myvalue: "Hello World"
  drink: coffee
```

### Overriding with `--set`

```bash
helm install solid-vulture ./mychart --dry-run=client --debug --set favoriteDrink=slurm
```

Now the rendered ConfigMap shows `drink: slurm` because the `--set` flag has higher precedence than the default `values.yaml`.

## Structured Values

You can nest values for better organization:

```yaml
favorite:
  drink: coffee
  food: pizza
```

Template usage:

```yaml
  drink: {{ .Values.favorite.drink }}
  food: {{ .Values.favorite.food }}
```

**Recommendation:** Keep the values tree **shallow** (favor flat structures) unless nesting is required for clarity or sub‑chart boundaries.

## Deleting a Default Key

To remove a key from the merged values, set it to `null` on the command line. This is helpful when a default block conflicts with a custom configuration.

Example (Drupal chart liveness probe):

```bash
helm install stable/drupal \
  --set image=my-registry/drupal:0.1.0 \
  --set livenessProbe.exec.command=[cat,docroot/CHANGELOG.txt] \
  --set livenessProbe.httpGet=null
```

Setting `livenessProbe.httpGet=null` tells Helm to delete that block before merging, preventing Kubernetes validation errors.

## Checklist
- [ ] Define default values in `values.yaml`.
- [ ] Use flat structures where possible.
- [ ] Override defaults with `-f` files or `--set` as needed.
- [ ] To delete a default key, set it to `null` in a higher‑precedence source.
- [ ] Remember the order of precedence when troubleshooting unexpected values.

Following these practices ensures predictable value handling across installs, upgrades, and rollbacks.
