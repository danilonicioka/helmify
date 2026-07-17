# Built-in Objects

Objects are passed into a template from the template engine. Your chart code can also pass objects around (e.g., with `with` and `range`). Some functions (e.g., `tuple`) can create new objects.

Objects can be simple (single value) or complex (containing other objects or functions). For example, the `Release` object contains several fields, and the `Files` object provides multiple helper functions.

## Release

The `Release` object describes the Helm release itself.

| Field | Description |
|-------|-------------|
| `Release.Name` | The release name |
| `Release.Namespace` | The namespace the release is deployed into (if the manifest doesn’t override) |
| `Release.IsUpgrade` | `true` if the current operation is an upgrade or rollback |
| `Release.IsInstall` | `true` if the current operation is an install |
| `Release.Revision` | The revision number for this release (starts at 1 on install) |
| `Release.Service` | The service rendering the template (always `Helm` for Helm) |

## Values

`Values` are the values passed into the template from `values.yaml` and any user‑supplied files. By default `Values` is empty.

## Chart

The `Chart` object contains the contents of `Chart.yaml`. Any key in `Chart.yaml` is accessible, e.g.:

```gotemplate
{{ .Chart.Name }}-{{ .Chart.Version }}
```

## Subcharts

`Subcharts` provides access to the scope of subcharts from the parent. Example:

```gotemplate
{{ .Subcharts.mySubChart.myValue }}
```

## Files

`Files` gives access to non‑special files in the chart (but not templates). Useful helper functions:

- `Files.Get "config.ini"` – Get a file as a string.
- `Files.GetBytes "image.png"` – Get file contents as a byte array (useful for images).
- `Files.Glob "*.txt"` – Return a list of files matching a glob pattern.
- `Files.Lines "list.txt"` – Iterate over a file line‑by‑line.
- `Files.AsSecrets` – Return file bodies as Base64‑encoded strings.
- `Files.AsConfig` – Return file bodies as a YAML map.

## Capabilities

`Capabilities` provides information about what the target Kubernetes cluster supports.

- `Capabilities.APIVersions` – Set of API versions. Use `{{ .Capabilities.APIVersions.Has "apps/v1/Deployment" }}` to test availability.
- `Capabilities.KubeVersion` – Kubernetes version information.
  - `Capabilities.KubeVersion.Version`
  - `Capabilities.KubeVersion.Major`
  - `Capabilities.KubeVersion.Minor`
- `Capabilities.HelmVersion` – Helm version details (same output as `helm version`).
  - `Capabilities.HelmVersion.Version`
  - `Capabilities.HelmVersion.GitCommit`
  - `Capabilities.HelmVersion.GitTreeState`
  - `Capabilities.HelmVersion.GoVersion`

## Template

`Template` contains information about the current template being rendered.

| Field | Description |
|-------|-------------|
| `Template.Name` | Namespaced file path to the current template (e.g., `mychart/templates/mytemplate.yaml`) |
| `Template.BasePath` | Path to the templates directory of the current chart (e.g., `mychart/templates/`) |

## Naming Conventions

Built‑in values always begin with a capital letter (following Go naming conventions). When defining your own values you can choose a style; many charts use lower‑case initial letters to distinguish them from built‑ins.

---

**Tip:** Use `{{ .Release.Name }}` and other built‑in objects to keep your templates declarative and portable across environments.
