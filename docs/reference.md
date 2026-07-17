# Reference Tables

## Image Registries Used by Helmify
| Registry | Type | Usage |
|----------|------|-------|
| `registry.access.redhat.com` | Red Hat official registry | Base images for container builds (e.g., `ubi8`)
| `tjpa-registry-quay-quay-enterprise.apps.ocp-hub.i.tj.pa.gov.br` | Private Quay registry | Mirror for any external images required by the project |

## Security Context Constraints (SCC) for Helmify
| SCC Name | Reason | Recommended Settings |
|----------|--------|----------------------|
| `restricted` | Default for non‑root pods | `runAsUser: 1000`, `runAsGroup: 1000`, `allowPrivilegeEscalation: false`, `capabilities: {drop: ["ALL"]}` |
| `anyuid` (optional) | When a container must run as root (rare) | Only grant to specific service accounts; avoid unless absolutely required |

## Helm Version Compatibility
| Helm Version | Supported by Helmify |
|--------------|----------------------|
| `>= v3.6.0` | Minimum required (as indicated in README) |
| `v3.8.x` | Fully tested (CI runs against this version) |
| `v3.9.x` | Verified – no breaking changes |

## Chart Structure Conventions (Helmify Specific)
| Directory/File | Description |
|----------------|-------------|
| `Chart.yaml` | Chart metadata – name, version, appVersion, keywords |
| `values.yaml` | User‑configurable defaults; global values under `global:` and component‑specific values under their own keys |
| `templates/` | Generated templates derived from files in `models/`; never edit directly – modify source in `models/` instead |
| `templates/_helpers.tpl` | Helper functions used across templates (e.g., fullname, labels) |
| `charts/` | Optional sub‑charts – keep minimal to avoid complexity |

These tables capture the core project‑specific conventions and configurations that developers should reference when extending Helmify or creating new Helm charts.
