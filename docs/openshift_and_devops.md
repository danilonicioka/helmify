# OpenShift, DevOps, and Resource Best Practices

This guide covers deployment practices for Red Hat OpenShift, DevOps workflows, RBAC constraints, Security Context Constraints (SCC), CRDs, and labelling standards.

---

## 1. OpenShift and Kustomize Integration

OpenShift environments introduce strict security and routing APIs (Routes, SecurityContextConstraints) that differ from vanilla Kubernetes.

### Kustomize vs. Helm
- **Kustomize**: Uses overlay-only patching, modifying raw manifests for specific target environments.
- **Helm**: Parametrizes configuration templates using a unified engine. 
*Helmify* translates Kustomize output and raw YAML manifests into standard templates, making it easy to migrate Kustomize projects to Helm.

---

## 2. Security Context Constraints (SCC) & Pod Templates

OpenShift utilizes Security Context Constraints (SCC) to validate pod security specifications.

### Guidelines for PodTemplates
- **Non-Root Execution**: Container images should run under non-privileged, non-root system users.
- **Dynamic UID Allocation**: Never hardcode user IDs (e.g. `runAsUser: 1001`) in pod specifications. OpenShift assigns a dynamic UID to namespaces under standard SCC policies. If an ID is hardcoded, the pod will be rejected.
- **Root Filesystem**: Set `readOnlyRootFilesystem: true` where possible, projecting temporary files into standard `emptyDir` volumes.

---

## 3. RBAC Best Practices (Least Privilege)

Role-Based Access Control configuration should restrict namespace execution limits.

- **Dedicated ServiceAccounts**: Create a dedicated `ServiceAccount` per workload component instead of using the default namespace account.
- **Scope Scrutiny**: Limit ClusterRoles and ClusterRoleBindings. Use localized `Roles` and `RoleBindings` scoped to the application namespace.
- **Credentials Decoupling**: ServiceAccounts must not leak tokens or private container registry auth secrets.

---

## 4. Custom Resource Definitions (CRDs)

In Helm 3, CRDs are treated as special resources.

### Guidelines
- CRD schemas must be placed inside the `crds/` directory at the chart root.
- CRD files must consist of plain YAML (they are not templated by the engine).

### Helm CRD Limitations
1. **No Updates**: Helm only installs CRDs if they do not already exist in the cluster. It will never patch, modify, or update existing CRDs during upgrades.
2. **No Deletions**: Helm will never delete CRDs upon uninstalling the chart, preventing accidental data loss of CRD instances.

---

## 5. Labels and Annotations Standard

Consistent labelling is critical for service discovery, selectors, and metadata queries.

- **Selector Labels**: Static values used by Services and Deployments to match pods (`app.kubernetes.io/name`, `app.kubernetes.io/instance`). They must remain constant.
- **Common Labels**: Set on all resources for accounting (`app.kubernetes.io/version`, `app.kubernetes.io/managed-by`, `helm.sh/chart`).
- **Annotations**: Store arbitrary non-identifying metadata (e.g., Prometheus scraping coordinates, rollout triggers, or OpenShift Topology annotations).

---

## 6. DevOps Best Practices

- **Configuration Decoupling**: Keep container configurations in ConfigMaps and secrets in Secrets, feeding them via `envFrom` rather than hardcoding variables in the Dockerfile or deployment template.
- **Image Pinning**: Always reference registry images with exact version tags or SHA hashes rather than mutable tags like `latest` in production.
- **Dry-run Validations**: Test integrations using `helm install --dry-run` to detect API validation issues before committing to staging or production namespaces.
