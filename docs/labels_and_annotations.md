# Labels and Annotations

This part of the Best Practices Guide discusses the best practices for using **labels** and **annotations** in your Helm chart.

## Is it a Label or an Annotation?

An item of metadata should be a **label** when **all** of the following conditions are true:

1. It is used by Kubernetes to **identify** this resource.
2. It is useful to expose to operators for the purpose of **querying** the system.

For example, we suggest using `helm.sh/chart: NAME-VERSION` as a label so that operators can conveniently find all instances of a particular chart.

If a piece of metadata is **not** used for querying, it should be set as an **annotation** instead.

> **Note:** Helm hooks are always stored as **annotations**.

## Standard Labels

The table below defines common labels that Helm charts use. Helm itself never requires a particular label to be present. Labels marked **REC** are *recommended* for global consistency, while **OPT** are *optional* – idiomatic but not strictly required.

| Name | Status | Description |
|------|--------|-------------|
| `app.kubernetes.io/name` | **REC** | This should be the app name, reflecting the entire app. Usually `{{ template "name" . }}` is used. It is used by many Kubernetes manifests and is not Helm‑specific. |
| `helm.sh/chart` | **REC** | Should contain the chart name and version: `{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}`. |
| `app.kubernetes.io/managed-by` | **REC** | Always set to `{{ .Release.Service }}`. Allows finding all resources managed by Helm. |
| `app.kubernetes.io/instance` | **REC** | Should be `{{ .Release.Name }}`. Helps differentiate between different instances of the same application. |
| `app.kubernetes.io/version` | **OPT** | The version of the app; can be set to `{{ .Chart.AppVersion }}`. |
| `app.kubernetes.io/component` | **OPT** | Marks the role of a component, e.g., `app.kubernetes.io/component: frontend`. |
| `app.kubernetes.io/part-of` | **OPT** | Indicates the higher‑level application when multiple charts are used together, e.g., a web app and its database. |

You can find more information on the Kubernetes labels prefixed with `app.kubernetes.io` in the official Kubernetes documentation.
