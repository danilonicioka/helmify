# Standardized Standardized Helm Chart

This Helm chart is a standardized template designed for applications deployed at **Organization**, especially when targeting **Red Hat OpenShift**. It serves as a base blueprint for developers, incorporating container security best practices and strict compliance with core architectural rules.

## Core Architectural Design Rules

This chart enforces a clean separation between the infrastructure blueprint (templates) and the operational overlay (`values.yaml`).

### Two-Tier Configuration Inheritance
Enforces a single source of truth using two distinct configuration levels:
1. **Global (Universal):** Managed in `values.yaml` under the `global` block (e.g., `TZ: "America/Belem"`). Generates a shared configmap and secret.
2. **Component-Specific:** Managed in `values.yaml` under each application/component block for variables unique to that container, defined in `cm` and `secret`.

### Deterministic Rollouts
**Immutable Config Strategy:** Any change to configurations in `values.yaml` triggers a rolling update using SHA256 checksums in the Pod template annotations:
- `checksum/configmaps`
- `checksum/secrets`

---

## Configuration Structure (`values.yaml`)

The `values.yaml` is organized into standardized sections for each component:

### 1. Core Workload Settings
Defines the `replicas`, custom `labels`, `annotations`, and the container `image` repository/tag.

### 2. Application Configuration
Defines environment variables for the container using two maps:
- `cm`: Non-sensitive configuration variables.
- `secret`: Sensitive variables.

### 3. Routing & Networking
Configures the internal Kubernetes `service` (ports and type) and OpenShift routes:
- **Default Route:** Internal route with self-signed TLS.
- **Internal Route:** Valid certificate for the internal Organization intranet (`*-internal.example.com`).
- **External Route:** Valid certificate for the external internet (`*example.com`).
- **Additional Routes:** A dynamic map allowing arbitrary extra routes to be generated alongside the standard ones.

### 4. Sidecars & Init Containers
Supports deploying arbitrary sidecars and initialization tasks:
- `extraContainers`: Dynamically injects sidecar containers into the Pod lifecycle.
- `initContainers`: Injects initialization containers that run prior to the main workload.
Both systems use a generic structural mapping that securely scopes `envFrom` (ConfigMap and Secret injection) and `volumeMounts` directly to the specific container, preventing sidecars from leaking configuration into the main global scope.

### 4. Resources
Defines CPU and Memory `requests` and `limits`.

### 5. Event-Driven Autoscaling (KEDA)
Optionally configure advanced event-driven autoscaling using KEDA by overriding the standard HPA configuration.

```yaml
keda:
  enabled: true
  minReplicas: 1
  maxReplicas: 5
  pollingInterval: 30
  cooldownPeriod: 300
  triggers:
    - type: rabbitmq
      metadata:
        queueName: "my-queue"
        queueLength: "5"
  triggerAuth:
    secretTargetRef:
      - parameter: host
        name: "my-secret"
        key: "RABBITMQ_URL"
```

### 6. Tiered "Fail-Fast" Health Probes
Standardized `tcpSocket` or `httpGet` probes with `initialDelaySeconds: 0`. Uses a generous `startupProbe` to allow slow applications to initialize, while keeping `livenessProbe` and `readinessProbe` dormant until ready.

### 7. Lifecycle & HA Strategy
Deployment strategies such as `RollingUpdate` (stateless apps) or `Recreate` (stateful applications using ReadWriteOnce persistence).

### 8. Persistence
Provides dynamic provisioning of PVCs. Supports ephemeral storage via `emptyDir` or persistent volumes via `storageRequest` and `mountPath`.

### 9. Scheduling & Node Assignment
Manages `imagePullSecrets`, `nodeSelector`, `tolerations`, and `affinity` rules (like podAntiAffinity to ensure pods are scheduled across different nodes).

### 10. Custom Files
Allows mounting arbitrary configuration files (like `nginx.conf` or keystores) directly from ConfigMaps or Secrets into the container via the `files` block.

---

## Usage

Other developers can copy the `<CHART_NAME>` folder, rename the references inside `Chart.yaml` and `values.yaml`, and start customising it immediately.

To lint and render templates during local development:
```bash
helm lint ./<CHART_NAME>
helm template ./<CHART_NAME>
```
