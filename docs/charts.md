# Charts

> **Warning**: This page has not yet been updated for Helm 4. Some of the content might be inaccurate or not applicable to Helm 4. For more information about the Helm 4 new features, improvements, and breaking changes, see Helm 4 Overview.

Helm uses a packaging format called charts. A chart is a collection of files that describe a related set of Kubernetes resources. A single chart might be used to deploy something simple, like a memcached pod, or something complex, like a full web app stack with HTTP servers, databases, caches, and so on.

Charts are created as files laid out in a particular directory tree. They can be packaged into versioned archives to be deployed.

If you want to download and look at the files for a published chart, without installing it, you can do so with `helm pull chartrepo/chartname`.

This document explains the chart format, and provides basic guidance for building charts with Helm.

## The Chart File Structure
A chart is organized as a collection of files inside of a directory. The directory name is the name of the chart (without versioning information). Thus, a chart describing WordPress would be stored in a `wordpress/` directory.

Inside of this directory, Helm will expect a structure that matches this:

```
wordpress/
  Chart.yaml          # A YAML file containing information about the chart
  LICENSE             # OPTIONAL: A plain text file containing the license for the chart
  README.md           # OPTIONAL: A human‑readable README file
  values.yaml         # The default configuration values for this chart
  values.schema.json  # OPTIONAL: A JSON Schema for imposing a structure on the values.yaml file
  charts/             # A directory containing any charts upon which this chart depends.
  crds/               # Custom Resource Definitions
  templates/          # A directory of templates that, when combined with values,
                      # will generate valid Kubernetes manifest files.
  templates/NOTES.txt # OPTIONAL: A plain text file containing short usage notes
```

Helm reserves use of the `charts/`, `crds/`, and `templates/` directories, and of the listed file names. Other files will be left as they are.

## The Chart.yaml File
The `Chart.yaml` file is required for a chart. It contains the following fields:

- **apiVersion**: The chart API version (required)
- **name**: The name of the chart (required)
- **version**: The version of the chart (required)
- **kubeVersion**: A SemVer range of compatible Kubernetes versions (optional)
- **description**: A single‑sentence description of this project (optional)
- **type**: The type of the chart (optional)
- **keywords**: A list of keywords about this project (optional)
- **home**: The URL of this project's home page (optional)
- **sources**: A list of URLs to source code for this project (optional)
- **dependencies**: A list of the chart requirements (optional)
- **maintainers**: A list of maintainers (optional)
- **icon**: URL to an SVG or PNG image (optional)
- **appVersion**: The version of the app that this contains (optional)
- **deprecated**: Whether this chart is deprecated (optional, boolean)
- **annotations**: Arbitrary annotations (optional)

> As of v3.3.2, additional fields are not allowed. Custom metadata should be added in `annotations`.

## Charts and Versioning
Every chart must have a version number following SemVer 2 (or a coerced form). The version is used by Helm commands and must match the package name, e.g., `nginx-1.2.3.tgz`.

## The apiVersion Field
Use `v2` for Helm 3‑compatible charts; `v1` for older Helm versions.

## The appVersion Field
This is informational only and should be quoted to avoid YAML parsing issues.

## The kubeVersion Field
Optional constraints on supported Kubernetes versions, supporting range syntax, wildcards, and operators.

## Deprecating Charts
Set `deprecated: true` in `Chart.yaml` and bump the version to deprecate a chart.

## Chart Types
- **application** (default): installable chart.
- **library**: provides utilities/functions, not installable unless `type: library` is set.

## Chart LICENSE, README and NOTES
- `LICENSE`: plain text license file.
- `README.md`: Markdown description, usage, values.
- `templates/NOTES.txt`: Short usage notes displayed after install/upgrade.

## Chart Dependencies
Define dependencies in `Chart.yaml` under `dependencies` or manage them manually in `charts/`.

### Managing Dependencies via `dependencies`
```yaml
dependencies:
  - name: apache
    version: 1.2.3
    repository: https://example.com/charts
```
Run `helm dependency update` to fetch them.

### Alias, Tags, and Condition Fields
- **alias**: rename a dependency.
- **tags**: group dependencies for bulk enable/disable.
- **condition**: YAML path that resolves to a boolean to enable/disable a dependency.

### Importing Child Values
Use `import-values` to propagate child chart values to the parent.

## Managing Dependencies Manually
Place unpacked charts inside the `charts/` directory (names must not start with `_` or `.`).

## Templates and Values
Templates are Go templates with Sprig functions. Values are supplied via `values.yaml` and/or `--set`.

## Predefined Values
- `Release.*`, `Chart.*`, `Files`, `Capabilities`

## Values Files and Global Values
You can define a `global` section to share values across subcharts.

## Schema Files
Optional `values.schema.json` to validate values. Can be disabled with `--skip-schema-validation`.

## CRDs
Place CRDs in `crds/`. They are installed before other resources and cannot be templated.

## Using Helm to Manage Charts
- `helm create mychart`
- `helm package mychart`
- `helm lint mychart`

## Chart Repositories
Charts are distributed via HTTP servers with an `index.yaml`.

## Chart Starter Packs
Use `helm create --starter` to bootstrap a chart from a starter pack.

---

*This document is part of the Helmify documentation hub.*
