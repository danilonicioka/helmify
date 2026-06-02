# Chart Model: Standardized TJPA Helm Chart

This Helm chart is a standardized, production-grade template designed for applications deployed at **Tribunal de Justiça do Pará (TJPA)**, especially when targeting **Red Hat OpenShift**. It serves as a base blueprint for developers, incorporating container security best practices and strict compliance with the core architectural rules.

## Core Architectural Design Rules

This chart implements the following 6 pillars of high-availability, maintainable container deployments:

### 1. Ultra-Lean Base Blueprint
The `templates/deployment.yaml` template acts purely as a lean infrastructure blueprint.
- Operational properties like `resources`, `strategy`, and health probes are entirely absent in their raw, opinionated form from the template core.
- Instead, they are completely parameterized and injected dynamically from `values.yaml` (which functions as the environment overlay).

### 2. Tiered "Fail-Fast" Health Probes
- Probes are configured in `values.yaml` (representing the overlay tier).
- Standardized on `tcpSocket` for absolute compatibility.
- Set `initialDelaySeconds: 0`.
- Utilizes the **Startup Probe** for initial warm-ups (configured with a safe timeout, e.g. 150s), keeping Liveness and Readiness probes completely dormant until the container is actually ready.

### 3. Three-Tier Configuration Inheritance
Enforces a single source of truth using three distinct configuration levels:
1. **Global (Universal):** Managed in `values.yaml` under the `global` block (e.g., `TZ: "America/Belem"`). Generates a shared configmap.
2. **Environment-Global:** Managed in `values.yaml` under `envGlobal` (e.g. `SPRING_PROFILES_ACTIVE`). Shared across all services in an environment.
3. **Service-Specific:** Managed in `values.yaml` under `serviceConfig` and `secrets` for variables unique to this container.

### 4. Clean Configuration Management
- No inline environment variables are hardcoded in the deployment manifest.
- The base deployment relies entirely on `envFrom` to hook into the tiered ConfigMaps and Secrets, ensuring full decoupling and seamless updates.

### 5. High-Availability & Lifecycle
- Configurable deployment strategies in `values.yaml` supporting:
  - `RollingUpdate` (default with `maxUnavailable: 0` and `maxSurge: 25%`) for standard stateless apps.
  - `Recreate` for components with `ReadWriteOnce` (RWO) storage constraints to avoid multi-attach lockouts.
- Standardizes on `terminationGracePeriodSeconds: 30` by default in production.

### 6. Deterministic Rollouts & Metadata
- **Immutable Config Strategy:** Any change to `.env` or configurations in `values.yaml` triggers a rolling update using SHA256 checksums in the Pod template annotations:
  - `checksum/global-config`
  - `checksum/env-global-config`
  - `checksum/service-config`
  - `checksum/secret`
- Standard `app.kubernetes.io` labels are configured in `_helpers.tpl` and propagated to resource metadata, selectors, and Pod templates automatically.
- Custom internal (`route-int.yaml`) and external (`route-ext.yaml`) OpenShift routes are provided, borrowing core TLS and annotation configs from the main route profile while allowing separate hosts and enablement checks.

---

## Usage

Other developers can copy the `chart-model` folder, rename the references inside `Chart.yaml` and `values.yaml`, and start customising it immediately.

To lint and render templates during local development:
```bash
helm lint ./chart-model
helm template ./chart-model
```
