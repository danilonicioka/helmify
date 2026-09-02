# Advanced Environment and Lifecycle Support

This document details how to support advanced Kubernetes native configuration options such as raw `env` arrays and `lifecycle` hooks within Helmify templates. 

While Helmify typically decouples configuration by extracting flat Key-Value pairs into ConfigMaps and Secrets (which are then injected into pods via `envFrom`), some advanced features require native schema arrays in the pod specification.

## Use Cases for Native `env` Arrays

You may need to bypass the `envFrom` logic and use a native `env` array in the following scenarios:
1. **Downward API Integration**: To inject metadata like the pod name, IP address, or namespace, Kubernetes strictly requires the `valueFrom: fieldRef` syntax, which can only be done in a native `env` array.
2. **Dependent Variable Interpolation**: To set variables that depend on other variables (e.g. `RABBITMQ_NODE_NAME: rabbit@$(MY_POD_NAME)`), Kubernetes interpolation requires the variables to be explicitly defined in the same `env` list. Variables injected via `envFrom` cannot be reliably interpolated.

## Use Cases for Native `lifecycle` Hooks

You may need to inject `lifecycle` hooks in the following scenarios:
1. **PreStop Hooks**: To allow for graceful shutdown, draining active connections, or delaying container termination during deployment rollouts (e.g., `sleep 15`).
2. **PostStart Hooks**: To execute initialization tasks or warm up caches immediately after the container starts.

## How to Implement in Templates

If you wish to enable support for these features, you must modify your deployment and cronjob templates in `models/single/templates/` and `models/multi/templates/`.

Add the following logic to the container specification in `deploy.yaml`, `deploy-api.yaml`, and `deploy-app.yaml`:

```gotemplate
          {{- if or (and $comp.truststore $comp.truststore.enabled) $comp.env }}
          env:
            {{- if and $comp.truststore $comp.truststore.enabled }}
            - name: JAVA_TOOL_OPTIONS
              value: {{ if hasKey (default dict $comp.cm) "JAVA_TOOL_OPTIONS" }}{{ printf "%v %s" (index $comp.cm "JAVA_TOOL_OPTIONS") $comp.truststore.javaToolOptions | quote }}{{ else }}{{ $comp.truststore.javaToolOptions | quote }}{{ end }}
            {{- end }}
            {{- if $comp.env }}
            {{- toYaml $comp.env | nindent 12 }}
            {{- end }}
          {{- end }}
```

And for `lifecycle` hooks, inject this block after the `resources` section:

```gotemplate
          {{- if $comp.lifecycle }}
          lifecycle:
            {{- toYaml $comp.lifecycle | nindent 12 }}
          {{- end }}
```

## How to Configure in `values.yaml`

Once the templates are updated to support them, you can define them in your `values.yaml` under any specific component block:

```yaml
api:
  env:
    - name: MY_POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: RABBITMQ_NODE_NAME
      value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local"
      
  lifecycle:
    preStop:
      exec:
        command: ["/bin/sh", "-c", "sleep 15"]
```
