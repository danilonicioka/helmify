# Template Guidelines

This document explains **how to add or modify Helm template files** in the `models/` directory so that Helmify can generate high‑quality charts.

## 1. Where to Put Templates
- **Single‑deployment charts** – use `models/single/`.
- **Multi‑deployment charts** – use `models/multi/`.
- Follow the existing folder structure (e.g. `templates/` sub‑folder) and keep filenames short and descriptive (e.g. `deploy-backend.yaml`).

## 2. Naming Conventions
- Use **kebab‑case** for filenames: `my-component.yaml`.
- Prefix component‑specific files with the component name, e.g. `deploy-backend.yaml`, `svc-backend.yaml`.
- **Do not include version numbers or hash suffixes**; Helmify will handle versioning via `values.yaml`.

## 3. Labels & Annotations
- Always include the **standard labels** defined in the project:
  ```yaml
  app.kubernetes.io/name: {{ include "{{ .Chart.Name }}.fullname" . }}
  app.kubernetes.io/component: {{ include "{{ .Chart.Name }}.fullname" . }}-{{ .Values.component }}
  app.kubernetes.io/part-of: {{ .Release.Namespace }}
  ```
- For OpenShift `Route` objects, use the **dynamic route association** pattern described in the *Helm, Kustomize & OpenShift Guide*.
- Keep **extraLabels** and **extraAnnotations** placeholders in each template so users can extend them via `values.yaml`.

## 4. Image References
- Reference images via **variables** in `values.yaml`:
  ```yaml
  image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
  ```
- Do **not hard‑code** registry URLs; the base registry is defined in the global `image.registry` value (default to the corporate Red Hat registry).

## 5. Security Context
- Include a **securityContext** block that enforces non‑root execution:
  ```yaml
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    allowPrivilegeEscalation: false
    capabilities:
      drop: ["ALL"]
  ```
- This aligns with the SCC constraints required on OpenShift.

## 6. Values File Structure
- Group component‑specific values under a top‑level key matching the component name:
  ```yaml
  backend:
    replicaCount: 2
    resources:
      limits:
        cpu: "500m"
        memory: "256Mi"
  ```
- Use **`global`** for shared configuration (e.g., common `imagePullSecrets`).

## 7. Testing Templates
- Run `helm lint ./tmp-chart` after generation to verify syntax.
- Use the `helmify -vv` flag for verbose output that shows which template files were used.

## 8. When Changing Existing Templates
- **Never edit generated templates directly** in the output chart; instead, modify the source files in `models/` and re‑run Helmify.
- Keep changes minimal – most customizations should be expressed via `values.yaml`.

---

Following these guidelines will keep the generated charts consistent, secure, and easy to maintain.
