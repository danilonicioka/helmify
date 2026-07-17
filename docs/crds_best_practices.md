# Custom Resource Definitions

This section of the **Best Practices Guide** covers creating and using **Custom Resource Definition** (CRD) objects.

## Two Distinct Pieces

1. **CRD Declaration** – The YAML file that has `kind: CustomResourceDefinition`.
2. **CRD Users** – Resources that reference the CRD, e.g., a CRD defines `foo.example.com/v1`. Any manifest with `apiVersion: example.com/v1` and `kind: Foo` is a resource that uses the CRD.

## Install a CRD Declaration Before Using the Resource

Helm loads resources into Kubernetes as quickly as possible. While Kubernetes can reconcile an entire set of manifests in one pass, **CRDs must be registered before any resources of that CRD’s kind can be applied**. Registration can take a few seconds, so the order matters.

---

## Method 1: Let Helm Do It for You

With Helm 3 the old `crd-install` hooks were removed. Instead, place your CRD YAML files (un‑templated) in a top‑level `crds/` directory of the chart:

- Helm automatically installs these CRDs on `helm install`.
- If the CRD already exists, Helm skips it and emits a warning.
- To skip CRD installation, use the `--skip-crds` flag.

### Caveats

- **No upgrade/delete support** – Helm currently cannot upgrade or delete CRDs. This is intentional to avoid accidental data loss.
- **`--dry-run` limitation** – Dry‑run cannot validate CRDs because the API server does not have the new types during the dry run. Consider using a separate chart for CRDs or `helm template` for validation.
- **Templating disabled** – CRDs in the `crds/` directory are not templated. This ensures Helm has a stable view of the cluster’s API set before rendering other templates.

---

## Method 2: Separate Charts

Create a dedicated chart that only contains the CRD definitions, and a second chart that contains the resources that use those CRDs.

- Install the CRD chart first, then install the dependent chart.
- This approach is useful for cluster operators who have admin privileges and want to manage CRDs separately from application workloads.

---

## Practical Recommendations

- **Keep CRDs in `crds/`** for most scenarios; it is the simplest and most idiomatic Helm 3 approach.
- **Version your CRDs** – When a new version of a CRD is required, increment the `metadata.annotations` `helm.sh/crd-version` (or similar) and document the migration steps.
- **Document the CRD** – Provide a `README.md` in the `crds/` directory that explains the purpose, schema, and any required RBAC.
- **Avoid templating** – Do not use Helm values inside CRDs; if you need dynamic content, generate the CRD via a separate chart or a pre‑install hook script.

---

## Example Structure

```text
mychart/
├─ Chart.yaml
├─ values.yaml
├─ templates/
│   └─ deployment.yaml
└─ crds/
    └─ myresource-crd.yaml   # <-- un‑templated CRD definition
```

**`myresource-crd.yaml`** (excerpt):
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.example.com
spec:
  group: example.com
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                size:
                  type: integer
  scope: Namespaced
  names:
    plural: myresources
    singular: myresource
    kind: MyResource
    shortNames:
    - mr
```

---

## Checklist

- [ ] Place all CRD YAML files (un‑templated) in the `crds/` directory.
- [ ] Ensure CRDs are versioned and documented.
- [ ] Do not rely on `--dry-run` to validate CRD usage.
- [ ] Consider a separate CRD chart if operators need explicit control over CRD lifecycle.

Following these guidelines will make your Helm charts more robust and easier to maintain, especially when dealing with custom resources.
