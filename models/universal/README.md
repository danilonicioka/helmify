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

# Tamper-proof workload governance: only listed workloads are permitted to deploy
allowedWorkloads:
  deploys:
    - api
    - app
  cronjobs: []

global:
  config:
    env:
      TZ: "America/Belem"
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

### 7. Tiered "Fail-Fast" Health Probes
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

### 8. Autoscaling (HPA & KEDA)
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

### 9. Init Containers (`initContainers`)
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
          requests:
            cpu: "10m"
            memory: "64Mi"
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

### 10. Sidecars (`extraContainers`)
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
          requests:
            cpu: "30m"
            memory: "64Mi"
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
        sharedVolumes:
          data:
            mountPath: "/var/lib/data"
            readOnly: true
```

---

### 11. Scheduled Batch Jobs (`cronjobs`)
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
      requests:
        cpu: "100m"
        memory: "256Mi"
    volumes:
      backup-storage:
        mountPath: "/mnt/backups"
        storageRequest: "20Gi"
        accessMode: "ReadWriteOnce"
```

---

### 12. OpenTelemetry Auto-Instrumentation
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

### 13. Subcomponents (Postgres & Redis)
Specialized subcomponents (`postgres`, `redis`) can be optionally generated into the chart (`templates/postgres.yaml`, `templates/redis.yaml`).
- **Defensive Rendering:** Templates safely guard all access against `nil` values so that charts can be linted or rendered cleanly with default `values.yaml` (`helm lint` and `helm template`) even when subcomponents are not explicitly enabled or configured.
- **Parity with Standardized Model:** Subcomponents support `shared:` resource opt-in (allowing database pods to consume shared ConfigMap/Secret environment variables and certificates/files), `service.enabled` (to disable external service exposition), decoupled `containerPort`, unified multi-volume management (`volumes:`), global private registry pull secrets (`global.imageCredentials`), and modern Vault Secrets Operator (VSO) integration with automated `ServiceAccount` and `VaultAuth` generation. Legacy `persistence:` remains supported in templates for backward-compatibility.

---

### 14. Workload Governance & Tamper-Proof Allowlist
To prevent developers from spinning up unauthorized workloads in GitOps environments where developers control their own `values` files (e.g., via ArgoCD pointing to developer Git repositories), Helmify automatically embeds an **immutable workload allowlist** into `templates/_helpers.tpl`:
- **Scope of Enforcement:**
  - **Deployments:** Restricts `.Values.deploys` strictly to components authorized during chart generation.
  - **CronJobs:** Restricts `.Values.cronjobs` strictly to scheduled jobs authorized during chart generation.
  - **InitContainers & Sidecars:** Restricts `.Values.deploys.<comp>.initContainers` and `.Values.deploys.<comp>.extraContainers` strictly to containers declared at chart generation.
- **Tamper-Proof Design:** The allowlist is encoded directly within the template layer (`<CHART_NAME>.workloadGovernance` helper) in the DevOps repository. Because it is not stored in `.Values`, it cannot be overridden, replaced, or bypassed by developer values files.
- **Fail-Fast Enforcement:** If an unauthorized workload or container is declared, Helm template rendering immediately halts with a `[SECURITY VIOLATION]` error, turning the ArgoCD synchronization status red and blocking OpenShift deployment.

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
