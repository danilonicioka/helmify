# Reference Guide and Conventions

This guide contains general naming conventions, YAML formatting rules, security references, and version compatibility tables for Helm and Helmify.

---

## 1. Naming & Versioning Conventions

### Chart Names
Chart names must follow DNS-1123 label conventions:
- Only lowercase letters, numbers, and dashes (`-`) are allowed.
- Must start and end with a lowercase letter or number.
- Cannot exceed 63 characters.

*Examples of invalid names*: `MyChart` (uppercase), `my_chart` (underscore), `my.chart` (dot), `mychart-` (trailing dash).

### Version Numbers
- **SemVer 2** must be used to represent chart and application version numbers.
- When SemVer versions are stored in Kubernetes labels, the `+` character must be replaced with `_`, as labels do not allow the `+` character.

---

## 2. Formatting & Design Conventions

- **YAML Indentation**: Always use two spaces (and never tabs) for indentation.
- **Namespace Property**: Avoid defining the `namespace` property in the `metadata` section of your chart templates. The namespace should be dynamically specified during deployment using the `--namespace` command-line flag or the release target.

---

## 3. Reference Tables

### Image Registries
| Registry | Type | Usage |
|----------|------|-------|
| `registry.access.redhat.com` | Red Hat Official Registry | Base images for container builds (e.g. `ubi-minimal`) |
| `tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br` | Private Quay Registry | Mirror for any external Docker Hub images to bypass rate limiting |

### Security Context Constraints (SCC)
| SCC Name | Scope | Recommended Settings |
|----------|-------|----------------------|
| `restricted` | Default non-root workloads | `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `capabilities: {drop: ["ALL"]}` |
| `anyuid` | Privilege escalation (Avoid) | Avoid unless required for specialized workloads. Needs cluster admin permission. |

### Helm Version Compatibility
- Minimum required Helm version: `>= v3.6.0`
- Fully verified and tested: `v3.8.x` and `v3.9.x`

### Chart Directory Conventions (Helmify Specific)
| Path | Purpose |
|------|---------|
| `Chart.yaml` | Chart metadata |
| `values.yaml` | User-configurable values defaults |
| `templates/` | Output templates (derived from raw input files or base models) |
| `templates/_helpers.tpl` | Shared naming/label helpers |
