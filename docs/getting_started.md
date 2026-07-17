# Getting Started

In this section of the guide, we'll create a chart and then add a first template. The chart we created here will be used throughout the rest of the guide.

## Charts

As described in the Charts Guide, Helm charts are structured like this:

```
mychart/
  Chart.yaml
  values.yaml
  charts/
  templates/
  ...
```

The `templates/` directory is for template files. When Helm evaluates a chart, it will send all of the files in the `templates/` directory through the template rendering engine. It then collects the results of those templates and sends them on to Kubernetes.

The `values.yaml` file is also important to templates. This file contains the default values for a chart. These values may be overridden by users during `helm install` or `helm upgrade`.

The `Chart.yaml` file contains a description of the chart. You can access it from within a template.

The `charts/` directory may contain other charts (which we call subcharts). Later in this guide we will see how those work when it comes to template rendering.

## A Starter Chart

For this guide, we'll create a simple chart called **mychart**, and then we'll create some templates inside of the chart.

```bash
$ helm create mychart
Creating mychart
```

### A Quick Glimpse of `mychart/templates/`

If you take a look at the `mychart/templates/` directory, you'll notice a few files already there:

- **NOTES.txt** – The "help text" for your chart. Displayed to users when they run `helm install`.
- **deployment.yaml** – A basic manifest for creating a Kubernetes Deployment.
- **service.yaml** – A basic manifest for creating a Service endpoint for your Deployment.
- **_helpers.tpl** – A place to put template helpers that you can re‑use throughout the chart.

For the tutorial we will **remove them all** so we can start from scratch:

```bash
$ rm -rf mychart/templates/*
```

> When writing production‑grade charts, keeping sensible defaults is useful; you may not want to delete the scaffolded files in real projects.

## A First Template

The first template we will create is a **ConfigMap**. ConfigMaps are simple key‑value stores that other resources (e.g., Pods) can consume.

Create the file `mychart/templates/configmap.yaml` with the following content:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mychart-configmap
  # The name above is static; we will later template it.

data:
  myvalue: "Hello World"
```

> **TIP:** Template names do not follow a rigid naming pattern. We recommend using the `.yaml` extension for YAML files and `.tpl` for helper templates.

Because this file lives in `templates/`, Helm will render it and submit the resulting manifest to Kubernetes.

## Installing the Chart

```bash
$ helm install full-coral ./mychart
```

You should see output similar to:

```
NAME: full-coral
LAST DEPLOYED: Tue Nov  1 17:36:01 2016
NAMESPACE: default
STATUS: DEPLOYED
REVISION: 1
TEST SUITE: None
```

You can view the rendered manifest with:

```bash
$ helm get manifest full-coral
```

Which will print something like:

```
---
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mychart-configmap
data:
  myvalue: "Hello World"
```

## Adding a Simple Template Call

Hard‑coding the `name:` field is not ideal; names should be unique per release. Update `configmap.yaml` to use the release name:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap

data:
  myvalue: "Hello World"
```

The `{{ .Release.Name }}` directive injects the release name into the manifest. Helm’s built‑in objects (like `Release`) are namespaced via the leading dot.

Re‑install the chart:

```bash
$ helm install clunky-serval ./mychart
```

Now the ConfigMap will be named `clunky-serval-configmap`.

## Rendering Without Installing

When you want to test the rendering without actually installing, use the `--dry-run` and `--debug` flags:

```bash
$ helm install --debug --dry-run goodly-guppy ./mychart
```

Helm will output the rendered manifests, e.g.:

```
---
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: goodly-guppy-configmap
data:
  myvalue: "Hello World"
```

While `--dry-run` is handy for checking template syntax, it does **not** guarantee that Kubernetes will accept the resources, because certain validation (especially involving CRDs) only occurs against a live cluster.

With this foundation you can explore more advanced Helm features in the subsequent sections of the guide.
