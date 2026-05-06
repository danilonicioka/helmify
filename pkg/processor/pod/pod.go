package pod

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/arttor/helmify/pkg/cluster"
	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	securityContext "github.com/arttor/helmify/pkg/processor/security-context"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const imagePullPolicyTemplate = "{{ .Values.%[1]s.%[2]s.imagePullPolicy }}"
const envValue = "{{ quote .Values.%[1]s.%[2]s.%[3]s.%[4]s }}"
const baseIndent = 8

const probeTemplate = `{{- with .Values.%[1]s }}
%[2]s:
  {{- toYaml . | nindent %[3]d }}
{{- end }}`

const numericTemplate = `{{- if not (kindIs "nil" .Values.%[1]s) }}
%[2]s: {{ .Values.%[1]s }}
{{- end }}`

func ProcessSpec(objName string, appMeta helmify.AppMetadata, spec corev1.PodSpec, addIndent int) (map[string]interface{}, helmify.Values, error) {
	nindent := baseIndent + addIndent

	values, err := processPodSpec(objName, appMeta, &spec)
	if err != nil {
		return nil, nil, err
	}

	// replace PVC to templated name
	for i := 0; i < len(spec.Volumes); i++ {
		vol := spec.Volumes[i]
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		tempPVCName := appMeta.TemplatedName(vol.PersistentVolumeClaim.ClaimName)

		spec.Volumes[i].PersistentVolumeClaim.ClaimName = tempPVCName
	}

	// replace container resources with template to values.
	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: unable to convert podSpec to map", err)
	}

	specMap, values, err = processNestedContainers(specMap, objName, values, "containers", nindent)
	if err != nil {
		return nil, nil, err
	}

	specMap, values, err = processNestedContainers(specMap, objName, values, "initContainers", nindent)
	if err != nil {
		return nil, nil, err
	}

	if appMeta.Config().ImagePullSecrets {
		if _, defined := specMap["imagePullSecrets"]; !defined {
			specMap["imagePullSecrets"] = "{{ .Values.imagePullSecrets | default list | toJson }}"
			values["imagePullSecrets"] = []string{}
		}
	}

	err = securityContext.ProcessContainerSecurityContext(objName, specMap, &values, nindent)
	if err != nil {
		return nil, nil, err
	}
	if spec.SecurityContext != nil {
		securityContextMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec.SecurityContext)
		if err != nil {
			return nil, nil, err
		}
		if len(securityContextMap) > 0 {
			err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%[1]s.podSecurityContext | nindent %d }}`, objName, nindent), "securityContext")
			if err != nil {
				return nil, nil, err
			}

			err = unstructured.SetNestedField(values, securityContextMap, objName, "podSecurityContext")
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to set deployment value field", err)
			}
		}
	}

	// process nodeSelector if presented:
	err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%s.nodeSelector | nindent %d }}`, objName, nindent), "nodeSelector")
	if err != nil {
		return nil, nil, err
	}
	if spec.NodeSelector != nil {
		err = unstructured.SetNestedStringMap(values, spec.NodeSelector, objName, "nodeSelector")
		if err != nil {
			return nil, nil, err
		}
	} else {
		err = unstructured.SetNestedField(values, map[string]interface{}{}, objName, "nodeSelector")
		if err != nil {
			return nil, nil, err
		}
	}

	// process tolerations if presented:
	err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%s.tolerations | nindent %d }}`, objName, nindent), "tolerations")
	if err != nil {
		return nil, nil, err
	}
	if spec.Tolerations != nil {
		tolerations := make([]any, len(spec.Tolerations))
		inrec, err := json.Marshal(spec.Tolerations)
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(inrec, &tolerations)
		if err != nil {
			return nil, nil, err
		}
		err = unstructured.SetNestedSlice(values, tolerations, objName, "tolerations")
		if err != nil {
			return nil, nil, err
		}
	} else {
		err = unstructured.SetNestedSlice(values, []any{}, objName, "tolerations")
		if err != nil {
			return nil, nil, err
		}
	}

	// process topologySpreadConstraints if presented:
	err = unstructured.SetNestedField(specMap, fmt.Sprintf(`{{- toYaml .Values.%s.topologySpreadConstraints | nindent %d }}`, objName, nindent), "topologySpreadConstraints")
	if err != nil {
		return nil, nil, err
	}
	if spec.TopologySpreadConstraints != nil {
		topologySpreadConstraints := make([]any, len(spec.TopologySpreadConstraints))
		inrec, err := json.Marshal(spec.TopologySpreadConstraints)
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(inrec, &topologySpreadConstraints)
		if err != nil {
			return nil, nil, err
		}
		err = unstructured.SetNestedSlice(values, topologySpreadConstraints, objName, "topologySpreadConstraints")
		if err != nil {
			return nil, nil, err
		}
	} else {
		err = unstructured.SetNestedSlice(values, []any{}, objName, "topologySpreadConstraints")
		if err != nil {
			return nil, nil, err
		}
	}

	return specMap, values, nil
}

func processNestedContainers(specMap map[string]interface{}, objName string, values map[string]interface{}, containerKey string, nindent int) (map[string]interface{}, map[string]interface{}, error) {
	containers, _, err := unstructured.NestedSlice(specMap, containerKey)
	if err != nil {
		return nil, nil, err
	}

	if len(containers) > 0 {
		containers, values, err = processContainers(objName, values, containerKey, containers, nindent)
		if err != nil {
			return nil, nil, err
		}

		err = unstructured.SetNestedSlice(specMap, containers, containerKey)
		if err != nil {
			return nil, nil, err
		}
	}

	return specMap, values, nil
}

func processContainers(objName string, values helmify.Values, containerType string, containers []interface{}, nindent int) ([]interface{}, helmify.Values, error) {
	for i := range containers {
		containerName := strcase.ToLowerCamel((containers[i].(map[string]interface{})["name"]).(string))
		var valuePath []string
		if containerName == objName || containerName == "" {
			valuePath = []string{objName}
		} else {
			valuePath = []string{objName, containerName}
		}
		valuePathStr := strings.Join(valuePath, ".")

		_, exists := (containers[i].(map[string]interface{}))["resources"]
		if exists {
			err := unstructured.SetNestedField(containers[i].(map[string]interface{}), fmt.Sprintf("[HELMIFY_WITH:%s.resources:%d]", valuePathStr, nindent+2), "resources")
			if err != nil {
				return nil, nil, err
			}
		}

		args, exists, err := unstructured.NestedStringSlice(containers[i].(map[string]interface{}), "args")
		if err != nil {
			return nil, nil, err
		}
		if exists && len(args) > 0 {
			err = unstructured.SetNestedField(containers[i].(map[string]interface{}), fmt.Sprintf(`{{- toYaml .Values.%s.args | nindent %d }}`, valuePathStr, nindent), "args")
			if err != nil {
				return nil, nil, err
			}

			err = unstructured.SetNestedStringSlice(values, args, append(valuePath, "args")...)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unable to set deployment value field", err)
			}
		}

		// Inject 3-Tier Probes templates using placeholders
		for _, pName := range []string{"startupProbe", "livenessProbe", "readinessProbe"} {
			err = unstructured.SetNestedField(containers[i].(map[string]interface{}), fmt.Sprintf("[HELMIFY_WITH:%s.%s:%d]", valuePathStr, pName, nindent+2), pName)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return containers, values, nil
}

func processPodSpec(name string, appMeta helmify.AppMetadata, pod *corev1.PodSpec) (helmify.Values, error) {
	values := helmify.Values{}
	for i, c := range pod.Containers {
		processed, err := processPodContainer(name, appMeta, c, &values)
		if err != nil {
			return nil, err
		}
		pod.Containers[i] = processed
	}

	for i, c := range pod.InitContainers {
		processed, err := processPodContainer(name, appMeta, c, &values)
		if err != nil {
			return nil, err
		}
		pod.InitContainers[i] = processed
	}

	for _, v := range pod.Volumes {
		if v.ConfigMap != nil {
			v.ConfigMap.Name = appMeta.TemplatedName(v.ConfigMap.Name)
		}
		if v.Secret != nil {
			v.Secret.SecretName = appMeta.TemplatedName(v.Secret.SecretName)
		}
	}
	pod.ServiceAccountName = fmt.Sprintf(`{{ include "%s.serviceAccountName" . }}`, appMeta.ChartName())

	for i, s := range pod.ImagePullSecrets {
		pod.ImagePullSecrets[i].Name = appMeta.TemplatedName(s.Name)
	}

	return values, nil
}

func processPodContainer(name string, appMeta helmify.AppMetadata, c corev1.Container, values *helmify.Values) (corev1.Container, error) {
	index := strings.LastIndex(c.Image, ":")
	if strings.Contains(c.Image, "@") && strings.Count(c.Image, ":") >= 2 {
		last := strings.LastIndex(c.Image, ":")
		index = strings.LastIndex(c.Image[:last], ":")
	}
	if index < 0 {
		return c, fmt.Errorf("wrong image format: %q", c.Image)
	}
	repo, tag := c.Image[:index], c.Image[index+1:]
	containerName := strcase.ToLowerCamel(c.Name)
	var valuePath []string
	if containerName == name || containerName == "" {
		valuePath = []string{name}
	} else {
		valuePath = []string{name, containerName}
	}
	valuePathStr := strings.Join(valuePath, ".")

	c.Image = fmt.Sprintf("{{ .Values.%[1]s.image.repository }}:{{ .Values.%[1]s.image.tag | default .Chart.AppVersion }}", valuePathStr)

	err := unstructured.SetNestedField(*values, repo, append(valuePath, "image", "repository")...)
	if err != nil {
		return c, fmt.Errorf("%w: unable to set deployment value field", err)
	}
	err = unstructured.SetNestedField(*values, tag, append(valuePath, "image", "tag")...)
	if err != nil {
		return c, fmt.Errorf("%w: unable to set deployment value field", err)
	}

	c, err = processEnv(name, containerName, appMeta, c, values)
	if err != nil {
		return c, err
	}

	for _, e := range c.EnvFrom {
		if e.SecretRef != nil {
			e.SecretRef.Name = appMeta.TemplatedName(e.SecretRef.Name)
		}
		if e.ConfigMapRef != nil {
			e.ConfigMapRef.Name = appMeta.TemplatedName(e.ConfigMapRef.Name)
		}
	}
	// Inject global configmap inheritance
	c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: fmt.Sprintf(`{{ include "%s.fullname" . }}-global`, appMeta.ChartName()),
			},
		},
	})
	c.Env = append(c.Env, corev1.EnvVar{
		Name:  cluster.DomainEnv,
		Value: fmt.Sprintf("{{ quote .Values.%s }}", cluster.DomainKey),
	})
	for k, v := range c.Resources.Requests {
		err = unstructured.SetNestedField(*values, v.ToUnstructured(), append(valuePath, "resources", "requests", k.String())...)
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container resources value", err)
		}
	}
	for k, v := range c.Resources.Limits {
		err = unstructured.SetNestedField(*values, v.ToUnstructured(), append(valuePath, "resources", "limits", k.String())...)
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container resources value", err)
		}
	}

	if c.ImagePullPolicy != "" {
		err = unstructured.SetNestedField(*values, string(c.ImagePullPolicy), append(valuePath, "imagePullPolicy")...)
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container imagePullPolicy", err)
		}
		c.ImagePullPolicy = corev1.PullPolicy(fmt.Sprintf("{{ .Values.%s.imagePullPolicy }}", valuePathStr))
	}

	c, err = processProbes(name, containerName, c, values)
	if err != nil {
		return c, err
	}

	return c, nil
}

func processProbes(name, containerName string, c corev1.Container, values *helmify.Values) (corev1.Container, error) {
	var valuePath []string
	if containerName == name || containerName == "" {
		valuePath = []string{name}
	} else {
		valuePath = []string{name, containerName}
	}

	processProbe := func(p *corev1.Probe, probeName string) error {
		if p == nil {
			// Default to tcpSocket if not present and container has ports
			if len(c.Ports) > 0 {
				p = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{
							Port: intstr.FromInt(int(c.Ports[0].ContainerPort)),
						},
					},
					InitialDelaySeconds: 0,
					PeriodSeconds:       10,
				}
			} else {
				// Create empty probe in values if not present to follow "Zero-Default Base"
				_ = unstructured.SetNestedField(*values, map[string]interface{}{}, append(valuePath, probeName)...)
				return nil
			}
		}
		pMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
		if err != nil {
			return err
		}
		// Force initialDelaySeconds to 0 per Best Practices
		pMap["initialDelaySeconds"] = int64(0)
		err = unstructured.SetNestedField(*values, pMap, append(valuePath, probeName)...)
		if err != nil {
			return err
		}
		return nil
	}

	if err := processProbe(c.LivenessProbe, "livenessProbe"); err != nil {
		return c, err
	}
	if err := processProbe(c.ReadinessProbe, "readinessProbe"); err != nil {
		return c, err
	}
	if err := processProbe(c.StartupProbe, "startupProbe"); err != nil {
		return c, err
	}

	// We'll surgically replace them in the Unstructured map later in processContainers
	return c, nil
}

func processEnv(name string, containerName string, appMeta helmify.AppMetadata, c corev1.Container, values *helmify.Values) (corev1.Container, error) {
	valuePath := []string{name, containerName}
	valuePathStr := strings.Join(valuePath, ".")
	for i := 0; i < len(c.Env); i++ {
		if c.Env[i].ValueFrom != nil {
			switch {
			case c.Env[i].ValueFrom.SecretKeyRef != nil:
				c.Env[i].ValueFrom.SecretKeyRef.Name = appMeta.TemplatedName(c.Env[i].ValueFrom.SecretKeyRef.Name)
			case c.Env[i].ValueFrom.ConfigMapKeyRef != nil:
				c.Env[i].ValueFrom.ConfigMapKeyRef.Name = appMeta.TemplatedName(c.Env[i].ValueFrom.ConfigMapKeyRef.Name)
			case c.Env[i].ValueFrom.FieldRef != nil, c.Env[i].ValueFrom.ResourceFieldRef != nil:
				// nothing to change here, keep the original value
			}
			continue
		}

		err := unstructured.SetNestedField(*values, c.Env[i].Value, append(valuePath, "env", strcase.ToLowerCamel(strings.ToLower(c.Env[i].Name)))...)
		if err != nil {
			return c, fmt.Errorf("%w: unable to set deployment value field", err)
		}
		c.Env[i].Value = fmt.Sprintf("{{ .Values.%s.env.%s }}", valuePathStr, strcase.ToLowerCamel(strings.ToLower(c.Env[i].Name)))
	}
	return c, nil
}

// AddReloadingAnnotations scans the PodSpec for ConfigMap and Secret references, and injects Helm checksum
// annotations into the provided map so that pods restart when configurations change.
func AddReloadingAnnotations(appMeta helmify.AppMetadata, annotations map[string]string, spec *corev1.PodSpec) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	configMaps := make(map[string]struct{})
	secrets := make(map[string]struct{})

	for _, v := range spec.Volumes {
		if v.ConfigMap != nil {
			configMaps[v.ConfigMap.Name] = struct{}{}
		}
		if v.Secret != nil {
			secrets[v.Secret.SecretName] = struct{}{}
		}
	}

	scanContainerRef := func(c corev1.Container) {
		for _, e := range c.EnvFrom {
			if e.ConfigMapRef != nil {
				configMaps[e.ConfigMapRef.Name] = struct{}{}
			}
			if e.SecretRef != nil {
				secrets[e.SecretRef.Name] = struct{}{}
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil {
				if e.ValueFrom.ConfigMapKeyRef != nil {
					configMaps[e.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
				}
				if e.ValueFrom.SecretKeyRef != nil {
					secrets[e.ValueFrom.SecretKeyRef.Name] = struct{}{}
				}
			}
		}
	}

	for _, c := range spec.Containers {
		scanContainerRef(c)
	}
	for _, c := range spec.InitContainers {
		scanContainerRef(c)
	}

	// Always add checksum for global configmap
	annotations["checksum/global-config"] = fmt.Sprintf(`{{ include (print $.Template.BasePath "/cm-global.yaml") . | sha256sum }}`)

	for cm := range configMaps {
		valueName := processor.ResolveValueName(appMeta, cm)
		annotations["checksum/config-"+valueName] = fmt.Sprintf(`{{ include (print $.Template.BasePath "/%s-configmap.yaml") . | sha256sum }}`, valueName)
	}
	for sec := range secrets {
		valueName := processor.ResolveValueName(appMeta, sec)
		annotations["checksum/secret-"+valueName] = fmt.Sprintf(`{{ include (print $.Template.BasePath "/%s-secret.yaml") . | sha256sum }}`, valueName)
	}

	// Filter out static placeholders from Kustomize
	for k, v := range annotations {
		if v == "$(CONFIG_HASH)" || v == "$(SECRET_HASH)" {
			delete(annotations, k)
		}
	}

	return annotations
}

// ReplacePlaceholders replaces HELMIFY_WITH placeholders with actual Helm templates.
func ReplacePlaceholders(s string) string {
	// 1. Handle single quotes: key: '[HELMIFY_WITH:path:indent]'
	r1 := regexp.MustCompile(`(?m)^(\s*)([a-zA-Z0-9]+):\s*'\[HELMIFY_WITH:([^:]+):([0-9]+)\]'`)
	s = r1.ReplaceAllString(s, "{{- with .Values.${3} }}\n${1}${2}:\n${1}  {{- toYaml . | nindent ${4} }}\n${1}{{- end }}")

	// 2. Handle block scalars if they occur: key: |-\n  [HELMIFY_WITH:path:indent]
	r2 := regexp.MustCompile(`(?m)^(\s*)([a-zA-Z0-9]+):\s*\|-\s*\n\s*\[HELMIFY_WITH:([^:]+):([0-9]+)\]`)
	s = r2.ReplaceAllString(s, "{{- with .Values.${3} }}\n${1}${2}:\n${1}  {{- toYaml . | nindent ${4} }}\n${1}{{- end }}")

	return s
}
