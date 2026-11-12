# Standardized Helm Chart

This Helm chart is a standardized model designed for applications deployed at **TJPA**, specifically targeting **Red Hat OpenShift**. It provides a hardened, production-ready blueprint adhering to container security best practices, strict OpenShift SCC compliance (non-root execution), and GitOps principles.

---

## 1. Core Architectural Design Rules

### Dual-File Configuration Architecture
Helmify generates two separate values files to cleanly decouple base blueprints from environment-specific configurations:
1. **`values.yaml` (Clean Base Skeleton):** Lives inside the chart package (`chart/values.yaml`). Contains the clean structural skeleton (`replicas`, base `image.repository`, service type, route definitions). It is intentionally clean and devoid of commented-out clutter.
2. **`values-env.yaml` (Operational Overlay):** Generated alongside the chart. Contains all environment-specific values (`config.env`, `secrets.env`, `resources`, `probes`, `autoscaling`, `scheduling`). Use this as the foundation to create `values-dev.yaml`, `values-hml.yaml`, or `values-prd.yaml`.

### Deterministic Rollouts
Any change to configurations in `config` or `secrets` triggers an automated rolling restart using SHA256 checksum annotations in the pod metadata:
- `checksum/cm-global` and `checksum/secret-global`
- `checksum/cm-<component>` and `checksum/secret-<component>`
- `checksum/init-containers-<component>`

---

## 2. Configuration Reference & Examples

### 1. Chart-Wide Options, Workload Governance & Private Registries
Configures global chart behavior, workload governance allowlist, and cluster-wide variables shared across all workloads.

```yaml
kubernetesClusterDomain: cluster.local
nameOverride: ""
fullnameOverride: ""

# Declarative workload governance: only listed workloads are permitted to deploy
allowedWorkloads:
  deploys:
    - api
    - app
  cronjobs: []

global:
  config:
    env:
      TZ: "America/Belem" # Managed globally for multi-deploy charts. For single-deploy charts, helmify automatically places TZ under deploys.<name>.config.env instead.
  secrets:
    env: {}
  imageCredentials:
    create: true
    registries:
      tjpa:
        registry: "tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br"
        username: "robot$my-robot"
        password: "" # Set securely in CI: --set global.imageCredentials.registries.tjpa.password=$PASSWORD
        email: "devops@tjpa.jus.br"
      dockerhub:
        registry: "docker.io"
        username: "my-user"
        password: ""
        email: ""
```

---

### 1.1 Shared Resources Pool (`shared`)
Allows defining ConfigMaps and Secrets (both environment variables and mounted files/certificates) that are shared across multiple deployments and cronjobs without duplication.

```yaml
shared:
  config:
    env:
      common-app-env:
        env:
          SPRING_PROFILES_ACTIVE: "prod"
    files:
      application.properties:
        mountPath: /app/config/application.properties
        content: "app.timeout=5000"

  secrets:
    env:
      pje-db:
        env:
          DB_USER: "smd_app"
          DB_PASSWORD: "secretpassword"
    files:
      # Single-file certificate:
      cert.p12:
        mountPath: /app/certs/cert.p12
        content: "..." # or stored in files/cert.p12
        b64enc: true

      # Multi-file bundle:
      tls-bundle:
        mountPath: /etc/ssl/certs
        files:
          tls.crt:
            content: "..."
          tls.key:
            content: "..."
```

#### Consuming Shared Resources in Deployments & CronJobs
Workloads opt into shared resources via their `shared:` section:

```yaml
deploys:
  api:
    shared:
      config:
        env:
          - name: common-app-env
        files:
          - name: application.properties
      secrets:
        env:
          - name: pje-db
        files:
          - name: cert.p12           # Mounts at default /app/certs/cert.p12
          - name: tls-bundle
            mountPath: /opt/ssl/certs # Optional: overrides default mountPath
```

---

### 2. Core Workload & OpenShift Topology
Defines the base deployment properties, container image, replicas, and OpenShift Console visualization.

```yaml
deploys:
  api:
    enabled: true
    # Optional: overrides the primary container name in the pod spec (defaults to deployment key e.g. 'api')
    containerName: "api"
    image:
      repository: "tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/sistemas/minha-api"
      tag: "1.0.0"
      pullPolicy: IfNotPresent
      pullSecrets: []
    replicas: 1
    command: []
    args: []
    labels:
      app.openshift.io/runtime: java
    annotations:
      # Link components visually in OpenShift Topology View:
      app.openshift.io/connects-to: '[{"apiVersion":"apps/v1","kind":"Deployment","name":"myapp-worker"}]'
```

---

### 3. Application Configuration & Custom Files (ConfigMaps & Secrets)
Environment variables and arbitrary configuration files injected into the container.

```yaml
    config:
      env:
        APP_ENV: "production"
        LOG_LEVEL: "INFO"
      files:
        # Inline file content:
        application.properties:
          mountPath: "/deployments/config/application.properties"
          content: |
            quarkus.http.port=8080
            quarkus.log.level=INFO
        # Automatically loaded from the chart's 'files/nginx.conf':
        nginx.conf:
          mountPath: "/etc/nginx/nginx.conf"

    secrets:
      env:
        DB_PASSWORD: "super-secret-password"
      files:
        # Binary or keystore file (b64enc: true prevents double-encoding):
        keystore.p12:
          mountPath: "/app/keystore/keystore.p12"
          b64enc: true
          content: "MIIKrAIBAzCC..."
```

---

### 4. Vault Secrets Operator (VSO / OpenBao)
Integrates natively with Vault Secrets Operator to fetch secrets directly from Vault/OpenBao using Kubernetes authentication.

```yaml
    vso:
      enabled: true
      role: "myapp-api-role"
      path: "kv/data/sistemas/myapp/api"
      files:
        # Mount specific Vault secret fields as individual files:
        vault-cert.p12:
          mountPath: "/app/certs/vault-cert.p12"
```

---

### 5. Routing & Networking
Configures internal Kubernetes Services and external OpenShift Routes.

```yaml
    service:
      enabled: true # Set to false for backend queue consumers/workers with no network endpoints
      type: ClusterIP
      ports:
        http:
          port: 8080
          containerPort: 8080 # Decoupled: container can listen on a different port than the service
        metrics:
          port: 9090
          protocol: TCP

    routes:
      default:
        enabled: true
        host: myapp.apps.ocp-dev.i.tj.pa.gov.br
        path: "/"
        annotations:
          haproxy.router.openshift.io/timeout: 60s
        tls:
          termination: edge
          insecureEdgeTerminationPolicy: Redirect
      internal:
        enabled: true
        host: myapp-int.apps.ocp-dev.i.tj.pa.gov.br
      external:
        enabled: false
        host: myapp.tj.pa.gov.br
      # Custom arbitrary route:
      admin-portal:
        enabled: true
        host: admin-myapp.apps.ocp-dev.i.tj.pa.gov.br
        path: "/admin"
        tls:
          termination: edge
          insecureEdgeTerminationPolicy: Redirect
```

---

### 6. Unified Multi-Volume Management
Supports persistent volume claims (PVC), ephemeral scratch storage (`emptyDir`), and existing PVC attachments.

```yaml
    volumes:
      # Dynamic persistent volume provisioned via StorageClass:
      data:
        enabled: true
        mountPath: "/var/lib/data"
        storageRequest: 5Gi
        accessMode: ReadWriteOnce
        storageClass: "ocs-storagecluster-ceph-rbd"
        ephemeral: false
      # Ephemeral emptyDir volume (scratch/cache disk):
      scratch:
        enabled: true
        mountPath: "/tmp/scratch"
        ephemeral: true
      # Mount a pre-existing PVC:
      shared-nfs:
        enabled: true
        mountPath: "/mnt/shared"
        existingClaim: "corp-shared-nfs-pvc"
```

---

### 7. Resource Management & Ephemeral Storage Enforcement
Defines container CPU, Memory, and Ephemeral Storage `requests` and `limits`.

```yaml
    resources:
      limits:
        cpu: "100m"
        memory: "512Mi"
        ephemeral-storage: "1Gi"
      requests:
        cpu: "30m"
        memory: "256Mi"
        ephemeral-storage: "500Mi"
```

#### Ephemeral Storage Limits (Node Protection & Eviction Prevention)
Ephemeral storage covers the container rootfs writable layer (such as `/tmp`), stdout/stderr container logs written to disk, and all `emptyDir` scratch volumes (`volumes.<name>.ephemeral: true`).
- **The Problem:** Without explicit ephemeral storage limits, a rogue container filling `/tmp` or generating unbuffered logs can exhaust the underlying node disk, triggering a cluster-wide `NodeDiskPressure` condition that causes the Kubelet to evict arbitrary healthy neighbor pods from the node.
- **The Solution:** By defining `limits.ephemeral-storage: "1Gi"`, the Kubelet monitors the container's scratch footprint and evicts **only the offending pod** when it exceeds the limit, protecting the node and other workloads.
- **Scheduling Protection:** Defining `requests.ephemeral-storage: "500Mi"` guarantees that Kubernetes schedules the pod only onto nodes with sufficient allocatable disk space, preventing silent disk overcommitment.
- **Customization:** Workloads requiring larger scratch space (such as batch report generation, large PDF processing, or document conversion) can safely override these limits in their operational overlay (`values-dev.yaml`, `values-prd.yaml`).

---

### 8. Tiered "Fail-Fast" Health Probes
Configured with zero initial delay and aggressive timeouts. A generous `startupProbe` allows slow applications (like Java/Spring Boot) to initialize without failing, while `livenessProbe` and `readinessProbe` remain dormant until startup succeeds.

```yaml
    probes:
      startup:
        tcpSocket:
          port: http
        initialDelaySeconds: 0
        periodSeconds: 5
        failureThreshold: 30
        successThreshold: 1
      liveness:
        tcpSocket:
          port: http
        initialDelaySeconds: 0
        periodSeconds: 10
        failureThreshold: 3
        successThreshold: 1
      readiness:
        tcpSocket:
          port: http
        initialDelaySeconds: 0
        periodSeconds: 10
        failureThreshold: 3
        successThreshold: 1
```

---

### 9. Autoscaling (HPA & KEDA)
Supports standard Horizontal Pod Autoscaler (CPU/Memory) or event-driven autoscaling with KEDA (e.g. RabbitMQ queues).

```yaml
    autoscaling:
      enabled: true
      engine: "keda" # Options: "hpa" or "keda"
      keda:
        minReplicas: 2
        maxReplicas: 10
        pollingInterval: 30
        cooldownPeriod: 300
        triggers:
          - type: rabbitmq
            metadata:
              queueName: "processos.tj.queue"
              queueLength: "150"
        triggerAuth:
          secretTargetRef:
            - parameter: host
              name: "myapp-secret"
              key: "RABBITMQ_URL"
      hpa:
        minReplicas: 1
        maxReplicas: 3
        metrics:
          - type: Resource
            resource:
              name: memory
              target:
                type: Utilization
                averageUtilization: 80
        behavior:
          scaleDown:
            stabilizationWindowSeconds: 120
          scaleUp:
            stabilizationWindowSeconds: 0
```

---

### 10. Init Containers (`initContainers`)
Initialization containers that execute sequentially before application containers start. Supports sharing the main component's volumes, dedicated scratch volumes, and scoped configs/secrets.

```yaml
    initContainers:
      wait-for-database:
        enabled: true
        image:
          repository: "tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/devops/curl"
          tag: "latest"
          pullPolicy: IfNotPresent
        command: ["sh", "-c", "until nc -z postgres 5432; do echo waiting for db; sleep 2; done;"]
        resources:
          limits:
            cpu: "100m"
            memory: "128Mi"
            ephemeral-storage: "500Mi"
          requests:
            cpu: "10m"
            memory: "64Mi"
            ephemeral-storage: "250Mi"
        # Share the main container's data volume:
        sharedVolumes:
          data:
            mountPath: "/var/lib/data"
        # Dedicated volume just for this init container:
        volumes:
          init-scratch:
            ephemeral: true
            mountPath: "/opt/init-data"
        config:
          files:
            init-config.json:
              mountPath: "/etc/config/init-config.json"
              content: |
                {"initialized": true}
```

---

### 11. Sidecars (`extraContainers`)
Auxiliary containers running alongside the main application pod (e.g., logging agents, proxies, workers). Scoped with independent ports, probes, and volumes.

```yaml
    extraContainers:
      oauth-proxy:
        enabled: true
        image:
          repository: "tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/devops/oauth-proxy"
          tag: "v7.5.0"
          pullPolicy: IfNotPresent
        command: []
        args: ["--config=/etc/proxy/proxy.cfg"]
        service:
          ports:
            proxy:
              port: 4180
        resources:
          limits:
            cpu: "200m"
            memory: "256Mi"
            ephemeral-storage: "500Mi"
          requests:
            cpu: "30m"
            memory: "64Mi"
            ephemeral-storage: "250Mi"
        probes:
          liveness:
            httpGet:
              path: "/ping"
              port: 4180
          readiness:
            httpGet:
              path: "/ready"
              port: 4180
        config:
          env:
            PROXY_COOKIE_SECURE: "true"
          files:
            proxy.cfg:
              mountPath: "/etc/proxy/proxy.cfg"
              content: |
                http_address = "0.0.0.0:4180"
                upstreams = [ "http://127.0.0.1:8080/" ]
        # Global environment inheritance (defaults to true):
        # Set to false to isolate third-party sidecars from global-cm and global-secret:
        inheritGlobal: false
        sharedVolumes:
          data:
            mountPath: "/var/lib/data"
            readOnly: true
```

> [!NOTE]
> **Global Environment Inheritance**: By default (`inheritGlobal: true`), all `extraContainers` automatically inherit `global-cm` and `global-secret` via `envFrom`. This ensures multi-container application workloads (e.g. specialized workers or sub-services in the same pod) share common database, storage, and organizational configurations. To completely isolate a third-party container (e.g. an oauth-proxy or exporter), explicitly set `inheritGlobal: false`.

---

### 12. Scheduled Batch Jobs (`cronjobs`)
Automated periodic batch workloads running via Kubernetes `CronJob`.

```yaml
cronjobs:
  nightly-database-backup:
    enabled: true
    schedule: "0 2 * * *" # Runs daily at 2:00 AM
    suspend: false
    concurrencyPolicy: Forbid # Allow, Forbid, Replace
    successfulJobsHistoryLimit: 3
    failedJobsHistoryLimit: 1
    restartPolicy: OnFailure
    image:
      repository: "tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br/devops/backup-agent"
      tag: "latest"
      pullPolicy: IfNotPresent
    command: ["/bin/backup.sh"]
    args: []
    config:
      env:
        BACKUP_DESTINATION: "/mnt/backups"
    secrets:
      env:
        S3_ACCESS_KEY: "secret-key"
    resources:
      limits:
        cpu: "500m"
        memory: "1Gi"
        ephemeral-storage: "1Gi"
      requests:
        cpu: "100m"
        memory: "256Mi"
        ephemeral-storage: "500Mi"
    volumes:
      backup-storage:
        mountPath: "/mnt/backups"
        storageRequest: "20Gi"
        accessMode: "ReadWriteOnce"
```

---

### 13. OpenTelemetry Auto-Instrumentation
Integrates directly with the OpenTelemetry Operator to inject tracing agents without modifying application code.

```yaml
    instrumentation:
      enabled: true
      language: java # Options: java, python, nodejs, go, dotnet
      exporter:
        endpoint: http://otel-collector-collector.tracing-system.svc.cluster.local:4318
      propagators:
        - tracecontext
        - baggage
        - b3
      env:
        OTEL_METRICS_EXPORTER: none
        OTEL_LOGS_EXPORTER: none
        OTEL_RESOURCE_ATTRIBUTES: deployment.environment=prd
```

---

### 14. Subcomponents (Postgres & Redis)
Specialized subcomponents (`postgres`, `redis`) can be optionally generated into the chart (`templates/postgres.yaml`, `templates/redis.yaml`).
- **Defensive Rendering:** Templates safely guard all access against `nil` values so that charts can be linted or rendered cleanly with default `values.yaml` (`helm lint` and `helm template`) even when subcomponents are not explicitly enabled or configured.
- **Parity with Standardized Model:** Subcomponents support `shared:` resource opt-in (allowing database pods to consume shared ConfigMap/Secret environment variables and certificates/files), `service.enabled` (to disable external service exposition), decoupled `containerPort`, unified multi-volume management (`volumes:`), global private registry pull secrets (`global.imageCredentials`), and modern Vault Secrets Operator (VSO) integration with automated `ServiceAccount` and `VaultAuth` generation. Legacy `persistence:` remains supported in templates for backward-compatibility.

---

### 15. Workload Governance & Declarative Allowlist
To prevent developers from spinning up rogue workloads in GitOps environments (e.g., via ArgoCD values injection), Helmify automatically populates an explicit **`allowedWorkloads`** allowlist in `values.yaml` and `values-env.yaml`:
- **Automated Discovery:** When Helmify generates the chart, it automatically extracts all detected deployment and cronjob names and generates the `allowedWorkloads` block:
  ```yaml
  allowedWorkloads:
    deploys:
      - api
      - worker
    cronjobs:
      - cleanup-cron
  ```
- **Declarative & Transparent:** Rather than embedding hidden rules in templates, the allowlist is explicit and fully transparent in values, maintaining GitOps best practices and standard Helm behavior.
- **Scope of Enforcement:**
  - **Deployments:** Restricts `.Values.deploys` strictly to components listed in `allowedWorkloads.deploys`.
  - **CronJobs:** Restricts `.Values.cronjobs` strictly to scheduled jobs listed in `allowedWorkloads.cronjobs`.
  - **Optional Container Granularity:** Also supports mapping format (e.g. `deploys.api.initContainers` / `extraContainers`) if granular container-level lockdown is required.
- **Fail-Fast Enforcement:** If an unauthorized deployment or cronjob is injected, Helm template rendering immediately halts with a `[SECURITY VIOLATION]` error, preventing rogue workloads from deploying onto OpenShift.

---

### 16. Workload Scheduling & High-Availability Pod Anti-Affinity
Ensures resilience and high availability by spreading deployment replicas across different cluster nodes:
- **Default HA Anti-Affinity:** Helmify automatically backfills `podAntiAffinity` under `scheduling.affinity` targeting the workload's component label (`app.kubernetes.io/component`), with `preferredDuringSchedulingIgnoredDuringExecution` (weight: 100) across `kubernetes.io/hostname`.
- **NodeSelector & Tolerations Preservation:** When an input manifest defines custom `nodeSelector` (e.g., node roles or zone constraints) or `tolerations`, Helmify retains the custom node constraints while preserving or backfilling the default `podAntiAffinity` so workloads do not lose high-availability spreading.
- **Custom Affinity Support:** Explicit user-defined `nodeAffinity`, `podAffinity`, or custom `podAntiAffinity` in input manifests are preserved.

---

## 3. Local Development & Linting

```bash
# Validate chart syntax
helm lint ./<CHART_NAME>

# Render templates with default values
helm template ./<CHART_NAME>

# Render templates with operational overlay
helm template ./<CHART_NAME> -f values-env.yaml
```
