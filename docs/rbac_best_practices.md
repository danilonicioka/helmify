# Role‑Based Access Control (RBAC)

This part of the **Best Practices Guide** discusses the creation and formatting of RBAC resources in Helm chart manifests.

## RBAC Resources
The following resources are considered RBAC objects:

- **ServiceAccount** (namespaced)
- **Role** (namespaced)
- **ClusterRole**
- **RoleBinding** (namespaced)
- **ClusterRoleBinding**

## YAML Configuration
Keep RBAC‑related configuration separate from ServiceAccount configuration. This separation makes the intent clear and avoids accidental coupling.

```yaml
rbac:
  # Whether RBAC resources (Roles, RoleBindings, etc.) should be created
  create: true

serviceAccount:
  # Whether a ServiceAccount should be created
  create: true
  # The name of the ServiceAccount to use. If omitted and `create` is true, a name is generated using the fullname template.
  name: ""
```

### Extending to Multiple Components
For charts that require multiple ServiceAccounts, nest the configuration under each component:

```yaml
someComponent:
  serviceAccount:
    create: true
    name: ""
anotherComponent:
  serviceAccount:
    create: true
    name: ""
```

## RBAC Resources Should Be Created by Default
- `rbac.create` should default to **true**. Users who prefer to manage RBAC themselves can set this to **false**.
- When `rbac.create` is **false**, the chart must still reference the ServiceAccount (if any) so that manually‑created RBAC objects can be linked later.

## Using RBAC Resources
- `serviceAccount.name` specifies the ServiceAccount name to be used by access‑controlled resources.
- If `serviceAccount.create` is **true**, the chart creates a ServiceAccount with the given name (or a generated one if the name is empty).
- If `serviceAccount.create` is **false** and a name is provided, the chart does **not** create a ServiceAccount but still references the specified name.
- If `serviceAccount.create` is **false** and no name is provided, the default ServiceAccount (`default`) is used.

## Helper Template for ServiceAccount Name
Add the following helper template to your chart’s `_helpers.tpl` (or equivalent) and reference it wherever a ServiceAccount name is needed:

```gotemplate
{{/*
Create the name of the ServiceAccount to use
*/}}
{{- define "mychart.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "mychart.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
```

Replace `mychart` with your chart’s actual name.

## Checklist
- [ ] Include separate `rbac` and `serviceAccount` sections in `values.yaml`.
- [ ] Default `rbac.create` to `true`.
- [ ] Provide a helper template for the ServiceAccount name.
- [ ] Ensure all RBAC resources reference `{{ include "mychart.serviceAccountName" . }}` where appropriate.
- [ ] Document any component‑specific ServiceAccount configurations.

Following these practices results in clear, maintainable RBAC configurations that respect the principle of least privilege while giving users flexible control over resource creation.
