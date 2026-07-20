# TJPA Helm Chart Standard Specification Guide

This guide establishes the strict specification and structural patterns for all Helm Charts within the TJPA (Tribunal de Justiça do Pará) ecosystem. It serves as the reference architecture for manual chart creation and the specification base for the `helmify` automation parser, wizard, and converter.

---

## 1. Directory Structure

All TJPA charts must conform to the following layout:

```
<chart-name>/
  Chart.yaml          # Metadata with annotations
  README.md           # Generated/updated reference documentation
  values.yaml         # Default configuration values (strict ordering)
  charts/             # Subcharts/dependencies (empty by default)
  crds/               # Plain Custom Resource Definitions (un-templated)
  templates/
    _helpers.tpl      # Central helper template library (no separate files)
    cm-global.yaml    # Global shared configuration ConfigMap
    cm.yaml           # Workload-specific configuration (if single model)
    deploy.yaml       # Deployment/StatefulSet template
    svc.yaml          # ClusterIP Service definition
    route-default.yaml# OpenShift Default Route (Self-Signed / OCP dev router)
    route-int.yaml    # OpenShift Internal Route (Intranet / TJPA intranet router)
    route-ext.yaml    # OpenShift External Route (Internet / TJPA internet router)
    secret.yaml       # Decoupled secrets manifest
```

---

## 2. Chart.yaml Specification

The `Chart.yaml` file must be API version `v2` (Helm 3+) and wrap string fields in quotes.

### Allowed Fields
- `apiVersion`: `"v2"` (Required)
- `name`: Chart name matching directory name (Required)
- `version`: SemVer 2 version of the chart (Required)
- `appVersion`: Version of the packaged application, wrapped in quotes (e.g. `"1.2.0"`) (Required)
- `description`: Single-sentence description of the chart (Optional)
- `type`: `"application"` or `"library"` (Defaults to `"application"`)
- `dependencies`: List of dependency charts (Optional)
- `annotations`: Custom metadata (Optional, as additional fields are rejected by Helm CLI v3.3.2+)

---

## 3. Values.yaml Structure & Field Ordering

To ensure consistency, the `values.yaml` must follow a strict section ordering, grouping variables logically from chart-wide/global configs to workload properties.

### Field Order and Default Values Pattern
1. **Section I: CHART-WIDE OPTIONS**
   - `kubernetesClusterDomain`: Default `"cluster.local"`.
   - `nameOverride` / `fullnameOverride`: Default `""`.
   - `global`: ConfigMaps/Secrets shared across all components (e.g. `TZ: "America/Belem"`).
2. **Section II: COMPONENT CONFIGURATION** (e.g. `backend`, `frontend`, or application name in single-model)
   - **Core Workload Settings**: 
     - `replicas: 0`
     - `connectsTo`: OpenShift topology mapping. Left commented out by default.
     - `labels: {}`
     - `annotations: {}`
     - Commented-out OpenShift Developer Console runtime properties within the standard workload `labels` map (`app.openshift.io/runtime`, etc.).
   - **Container Image**: `image.repository` (pointing to Red Hat Quay or UBI-based private registry), `image.tag` (default `"latest"`), `image.pullPolicy` (`IfNotPresent`).
   - **Application Config (Decoupled)**: `cm: {}`, `secret: {}`.
   - **Routing**: `route.annotations`, `route.tls.termination` (`edge`), `route.tls.insecureEdgeTerminationPolicy` (`Redirect`), `route.path`.
     - `route.default.enabled: true` with dev cluster domain host.
     - `route.internal.enabled: false` with internal intranet domain host.
     - `route.external.enabled: false` with internet zone domain host.
   - **Service Port**: `service.type: ClusterIP`, `service.port: 8080`, `service.targetPort: 8080`.
   - **Resources**: `resources: {}` (empty map by default, commented example for CPU/Memory constraints).
   - **Probes**: `startupProbe: {}`, `livenessProbe: {}`, `readinessProbe: {}` (empty by default).
   - **Lifecycle & HA**: `strategy` (RollingUpdate details), `terminationGracePeriodSeconds: 30`.
3. **Section III: SCHEDULING & COMMON CONFIGS**
   - `imagePullSecrets`, `nodeSelector`, `tolerations`, `affinity` (standard pod anti-affinity on hostname).

### Self-Documenting Values & Commented-Out Placeholders

To maximize developer convenience and establish clear parameters, all customizable or advanced blocks within the default `values.yaml` file must be accompanied by **commented-out examples**. This eliminates the need to constantly look up API structures or schema maps.

#### 1. Configuration Blocks to Document
The following keys inside `values.yaml` must always include commented-out examples showing typical configurations:
- **Labels & Annotations**: Placeholders demonstrating console runtimes and topology links:
  ```yaml
    # -- Custom labels to add to the backend deployment/pods
    labels: {}
      # app.openshift.io/runtime: openjdk
    # -- Custom annotations to add to the backend deployment 
    annotations: {}
      # app.openshift.io/connects-to: '[{"apiVersion":"apps/v1","kind":"Deployment","name":"db"}]'
  ```
- **Operational Resources**: Showing standard CPU/Memory requests and limits:
  ```yaml
    resources: {}
      # limits:
      #   cpu: 500m
      #   memory: 512Mi
      # requests:
      #   cpu: 100m
      #   memory: 256Mi
  ```
- **Health Probes**: Demonstrating zero-delay `tcpSocket` specifications:
  ```yaml
    startupProbe: {}
    # startupProbe:
    #   tcpSocket:
    #     port: 8080
    #   initialDelaySeconds: 0
    #   periodSeconds: 5
    #   failureThreshold: 30
  ```
- **Deployment Strategy**: Outlining standard rolling updates properties:
  ```yaml
    strategy: {}
      # type: RollingUpdate
      # rollingUpdate:
      #   maxSurge: 25%
      #   maxUnavailable: 0
  ```

#### 2. Converter & Wizard Preservation
When using automation pipelines or the generation wizards:
- If these keys are left empty by the developer/wizard, the active value must be rendered as an empty structure (e.g. `labels: {}` or `resources: {}`), leaving the commented lines intact directly underneath them in the output.
- If the developer provides actual values, the automation parser/wizard will dynamically uncomment the structure, inject the specified parameters, and write them into the rendered `values.yaml`.


---

## 4. Helper Templates (`_helpers.tpl`)

All labels and annotations must be defined inside the unified `_helpers.tpl` file. No separate helper files (e.g. `backend_helpers.tpl`) are allowed.

### Core Helpers

1. **`chart.fullname`**: Derives a DNS-compliant release/chart name truncated to 63 characters.
2. **`chart.labels`**: Outputs base metadata (helm.sh/chart, name, instance, version, managed-by).
3. **`chart.selectorLabels`**: Returns name and instance labels.
4. **`component.labels` / `component.selectorLabels`** (e.g., `chart-model-multi.backend.labels`):
   - Merges base labels with custom component-specific values from the values file.
   - Sets the component label dynamically using the format: 
     `app.kubernetes.io/component: {{ include "<chart>.fullname" . }}-<component-name>` (e.g. `release-name-chart-model-multi-backend`).

---

## 5. Workload Manifest (`deploy.yaml`) Standards

Deployments and StatefulSets must follow these operational guidelines:

### Labels and Annotations
- **Metadata Labels**: Must reference the component labels helper:
  ```yaml
  labels:
    {{- include "chart-model-multi.backend.labels" . | nindent 4 }}
  ```
- **Metadata Annotations**: Configured using helper definitions, merging global annotations and component-specific keys:
  ```yaml
  {{- $annotations := include "chart-model-multi.backend.annotations" . }}
  {{- if $annotations }}
  annotations:
    {{- $annotations | nindent 4 }}
  {{- end }}
  ```
- **Rollout Checksums (Pod Template Annotations)**:
  Inline SHA256 checksums of ConfigMaps and Secrets must be calculated to trigger automatic pod rollouts when configuration changes:
  ```yaml
  checksum/cm-config: '{{ include (print $.Template.BasePath "/cm.yaml") . | sha256sum }}'
  ```

### Non-Root Constraints & SCC Compliance
- Container workloads must never run as root.
- Do not hardcode specific uid/gid constraints. Allow OpenShift Security Context Constraints (SCC) to automatically assign the uid dynamically.

### Decoupled Configuration (`envFrom`)
All ConfigMaps (`cm`) and Secrets (`secret`) defined in values are projected as environment variables via `envFrom`:
```yaml
envFrom:
  - configMapRef:
      name: {{ include "chart.fullname" . }}-global
  - configMapRef:
      name: {{ include "chart.fullname" . }}-cm
```

### Probes & Resources
- Probes are initialized conditionally if values are defined, preferring `tcpSocket` with `initialDelaySeconds: 0` for instant readiness discovery.
- Resources must stay under an operational overlay and default to empty.

---

## 6. Route Definition & The 3 Variations

To accommodate the secure routing layers at TJPA, every chart template must include three Route files corresponding to default, intranet, and internet zones:

### A. Default Route (`route-default.yaml`)
- **Router**: OpenShift default router (often self-signed certificates).
- **Target**: Internal developers/tools.
- **Default**: `enabled: true`
- **Host Example**: `<app-name>.apps.ocp-dev.i.tj.pa.gov.br`

### B. Internal Route (`route-int.yaml`)
- **Router**: TJPA intranet router (trusted corporate wildcard certificate).
- **Target**: Internal TJPA employees/intranet subnet.
- **Default**: `enabled: false`
- **Host Example**: `<app-name>-i.tjpa.jus.br`

### C. External Route (`route-ext.yaml`)
- **Router**: TJPA internet DMZ router (publicly signed SSL wildcard certificate).
- **Target**: External public internet clients.
- **Default**: `enabled: false`
- **Host Example**: `<app-name>.tjpa.jus.br`

### Route Template Structure
Routes are configured with Edge SSL termination:
```yaml
{{- if .Values.component.route.default.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "chart.fullname" . }}-default
  labels:
    {{- include "chart.component.labels" . | nindent 4 }}
spec:
  host: {{ .Values.component.route.default.host }}
  path: {{ .Values.component.route.path }}
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  to:
    kind: Service
    name: {{ include "chart.fullname" . }}
{{- end }}
```
