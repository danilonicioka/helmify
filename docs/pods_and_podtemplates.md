# Pods and PodTemplates

This part of the **Best Practices Guide** discusses how to format the *Pod* and *PodTemplate* sections in Helm chart manifests.

## Resources that use PodTemplates

The following (non‑exhaustive) Kubernetes resources embed a `PodTemplate` specification:

- **Deployment**
- **ReplicationController**
- **ReplicaSet**
- **DaemonSet**
- **StatefulSet**

## Images

- A container image should use a **fixed tag** or the **SHA** of the image. Avoid tags such as `latest`, `head`, `canary`, or any other *floating* tags.
- Define images in `values.yaml` to make them easy to swap out, e.g.:

```yaml
image: {{ .Values.redisImage | quote }}
```

- You may also separate the image name and tag into two distinct values:

```yaml
image: "{{ .Values.redisImage }}:{{ .Values.redisTag }}"
```

## ImagePullPolicy

Helm’s `helm create` scaffold sets `imagePullPolicy` to **IfNotPresent** by default:

```yaml
imagePullPolicy: {{ .Values.image.pullPolicy }}
```

Corresponding excerpt from `values.yaml`:

```yaml
image:
  pullPolicy: IfNotPresent
```

Kubernetes also defaults `imagePullPolicy` to **IfNotPresent** when the field is omitted. If a different policy is required (e.g., `Always`), simply change the value in `values.yaml`.

## PodTemplates Should Declare Selectors

Every `PodTemplate` section should include an explicit **selector** that matches the labels applied to the pod template. This creates a clear relationship between the controller and the pods it manages.

**Example:**

```yaml
selector:
  matchLabels:
    app.kubernetes.io/name: MyName
template:
  metadata:
    labels:
      app.kubernetes.io/name: MyName
```

### Why Selectors Matter

- For controllers like **Deployment**, the selector determines which pods belong to the set. If you rely on a label that changes (e.g., version or release date), the controller may lose track of its pods, leading to orphaned resources or rollout failures.
- Explicit selectors make the chart more deterministic and easier to reason about, especially when multiple versions of an application coexist.

## Summary Checklist

- ✅ Use fixed image tags or SHA digests.
- ✅ Define images and `pullPolicy` in `values.yaml`.
- ✅ Include a selector that matches the pod template’s labels.
- ✅ Keep selector labels stable (avoid version‑specific labels).

Following these guidelines will produce more reliable Helm charts that behave predictably across upgrades and rollbacks.
