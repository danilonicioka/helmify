# Debugging Templates

Helm templates can be difficult to troubleshoot because rendering errors are often surfaced only when the Kubernetes API server rejects the generated YAML. Below are common commands and techniques to help you debug Helm charts effectively.

---
## Core Debug Commands
| Command | Description |
|---|---|
| `helm lint <chart>` | Performs static analysis of the chart, checking for common best‑practice violations and syntax errors. |
| `helm template <chart> --debug` | Renders all templates locally and prints the output with detailed debugging information (including the values used). |
| `helm install <release> <chart> --dry-run --debug` | Renders templates locally **and** performs a client‑side validation of the manifest. No resources are created. |
| `helm install <release> <chart> --dry-run=server --debug` | Same as above but also performs a server‑side dry‑run, executing any `lookup` calls against the cluster. |
| `helm get manifest <release>` | Retrieves the manifest that is currently installed for a given release (useful after a real install). |

---
## Quick Work‑around for YAML Parse Errors
If a particular section of a template causes a YAML parsing failure, you can comment it out and re‑run a dry‑run to see the rest of the rendered output.

```yaml
apiVersion: v2
# problematic: {{ .Values.foo | quote }}
```

Running `helm install --dry-run --debug` will output the file with the commented line preserved, allowing you to inspect the surrounding generated content.

---
## Tips & Best Practices
- **Use `--debug`**: It adds the `{{- printf "%s" . }}` style debug output and shows the value context for each template.
- **Validate with `kubectl apply --dry-run=client`**: After rendering, pipe the output to `kubectl apply --dry-run=client -f -` to let `kubectl` perform its own YAML validation.
- **Check rendered output with `helm template`**: Save the output to a file and examine it with a YAML linter or IDE.
- **Leverage `helm lint` rules**: Extend linting with custom rules via the `--strict` flag for stricter checks.
- **Inspect lookups**: When using `lookup`, run with `--dry-run=server` to ensure the cluster can resolve the queries.

---
## Example Debug Session
```bash
# 1. Lint the chart
helm lint mychart

# 2. Render with values and debug output
helm template mychart --debug > rendered.yaml

# 3. Validate the rendered manifest with kubectl
kubectl apply --dry-run=client -f rendered.yaml

# 4. If a specific template fails, comment the offending line and re‑run:
# (edit mychart/templates/configmap.yaml)
#   # {{ .Values.badKey | quote }}
helm install myrelease mychart --dry-run --debug
```

---
## Summary
- Use **lint**, **template**, and **dry‑run** commands to isolate issues.
- Comment problematic sections to view partial output.
- Combine Helm’s debug output with `kubectl` validation for comprehensive checks.
- Remember that server‑side dry‑run validates `lookup` calls against the cluster.

Refer to the **Flow Control** and **Variables** sections for additional context on template scope and debugging techniques.
