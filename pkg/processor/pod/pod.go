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
	yamlformat "github.com/arttor/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

	// Build a map of PVC-backed volumes for quick lookup
	pvcVolumes := make(map[string]struct{})
	for _, vol := range spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcVolumes[vol.Name] = struct{}{}
		}
	}

	// replace PVC, ConfigMap and Secret to templated name
	for i := 0; i < len(spec.Volumes); i++ {
		vol := spec.Volumes[i]
		if vol.PersistentVolumeClaim != nil {
			tempPVCName := appMeta.TemplatedName(vol.PersistentVolumeClaim.ClaimName)
			spec.Volumes[i].PersistentVolumeClaim.ClaimName = tempPVCName
			spec.Volumes[i].Name = fmt.Sprintf("[HELMIFY_PVC_VOL:%s:%s]", objName, vol.Name)
		}
		if vol.ConfigMap != nil {
			spec.Volumes[i].ConfigMap.Name = ResolveConfigMapVolumeName(appMeta, vol.ConfigMap.Name, objName)
		}
		if vol.Secret != nil {
			spec.Volumes[i].Secret.SecretName = ResolveSecretVolumeName(appMeta, vol.Secret.SecretName, objName)
		}
	}

	// Update container and initContainer volume mounts to placeholder if PVC-backed
	updateMounts := func(containers []corev1.Container) {
		for i := range containers {
			for j := range containers[i].VolumeMounts {
				mountName := containers[i].VolumeMounts[j].Name
				if _, ok := pvcVolumes[mountName]; ok {
					containers[i].VolumeMounts[j].Name = fmt.Sprintf("[HELMIFY_PVC_MOUNT:%s:%s]", objName, mountName)
				}
			}
		}
	}
	updateMounts(spec.Containers)
	updateMounts(spec.InitContainers)

	// replace container resources with template to values.
	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&spec)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: unable to convert podSpec to map", err)
	}

	specMap, values, err = processNestedContainers(specMap, objName, values, "containers", nindent, appMeta)
	if err != nil {
		return nil, nil, err
	}

	specMap, values, err = processNestedContainers(specMap, objName, values, "initContainers", nindent, appMeta)
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

	if spec.TerminationGracePeriodSeconds != nil {
		err = unstructured.SetNestedField(specMap, fmt.Sprintf("[HELMIFY_GRACE_PERIOD:%s]", objName), "terminationGracePeriodSeconds")
		if err != nil {
			return nil, nil, err
		}
		err = unstructured.SetNestedField(values, *spec.TerminationGracePeriodSeconds, objName, "terminationGracePeriodSeconds")
		if err != nil {
			return nil, nil, err
		}
	}

	return specMap, values, nil
}

func processNestedContainers(specMap map[string]interface{}, objName string, values map[string]interface{}, containerKey string, nindent int, appMeta helmify.AppMetadata) (map[string]interface{}, map[string]interface{}, error) {
	containers, _, err := unstructured.NestedSlice(specMap, containerKey)
	if err != nil {
		return nil, nil, err
	}

	if len(containers) > 0 {
		containers, values, err = processContainers(objName, values, containerKey, containers, nindent, appMeta)
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

func processContainers(objName string, values helmify.Values, containerType string, containers []interface{}, nindent int, appMeta helmify.AppMetadata) ([]interface{}, helmify.Values, error) {
	for i := range containers {
		containerName := strcase.ToLowerCamel((containers[i].(map[string]interface{})["name"]).(string))
		var valuePath []string
		if containerName == objName || containerName == "" || len(containers) == 1 {
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

		// Inject standardized envFrom block using placeholder
		kebabName := processor.NormalizeComponentName(objName)
		err = unstructured.SetNestedField(containers[i].(map[string]interface{}), fmt.Sprintf("[HELMIFY_ENV_FROM:%s:%s:%d]", objName, kebabName, nindent), "envFrom")
		if err != nil {
			return nil, nil, err
		}
	}
	return containers, values, nil
}

func processPodSpec(name string, appMeta helmify.AppMetadata, pod *corev1.PodSpec) (helmify.Values, error) {
	values := helmify.Values{}
	for i, c := range pod.Containers {
		processed, err := processPodContainer(name, appMeta, c, &values, i == 0 && len(pod.Containers) == 1)
		if err != nil {
			return nil, err
		}
		pod.Containers[i] = processed
	}

	for i, c := range pod.InitContainers {
		processed, err := processPodContainer(name, appMeta, c, &values, false)
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

func processPodContainer(name string, appMeta helmify.AppMetadata, c corev1.Container, values *helmify.Values, isPrimary bool) (corev1.Container, error) {
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
	if containerName == name || containerName == "" || isPrimary {
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

	// We clear envFrom as it will be handled by the standardized block injected in processContainers
	c.EnvFrom = nil

	c.Env = append(c.Env, corev1.EnvVar{
		Name:  cluster.DomainEnv,
		Value: fmt.Sprintf("{{ quote .Values.%s }}", cluster.DomainKey),
	})
	// Zero-Default initialization handled below

	// Initialize as empty object {} per Zero-Default standard
	err = unstructured.SetNestedField(*values, map[string]interface{}{}, append(valuePath, "resources")...)
	if err != nil {
		return c, err
	}
	if b, err := json.Marshal(c.Resources); err == nil {
		var resMap map[string]interface{}
		if err := json.Unmarshal(b, &resMap); err == nil {
			cleanMap(resMap)
			if len(resMap) > 0 {
				resYaml, err := yamlformat.Marshal(map[string]interface{}{"resources": resMap}, 0)
				if err == nil {
					registryKey := "resources." + strcase.ToLowerCamel(strings.Join(valuePath, "."))
					helmify.OriginalValuesRegistry.Store(registryKey, strings.TrimSpace(resYaml))
				}
			}
		}
	}

	if c.ImagePullPolicy != "" {
		err = unstructured.SetNestedField(*values, string(c.ImagePullPolicy), append(valuePath, "imagePullPolicy")...)
		if err != nil {
			return c, fmt.Errorf("%w: unable to set container imagePullPolicy", err)
		}
		c.ImagePullPolicy = corev1.PullPolicy(fmt.Sprintf("{{ .Values.%s.imagePullPolicy }}", valuePathStr))
	}

	c, err = processProbes(name, containerName, c, values, isPrimary)
	if err != nil {
		return c, err
	}

	return c, nil
}

func processProbes(name, containerName string, c corev1.Container, values *helmify.Values, isPrimary bool) (corev1.Container, error) {
	var valuePath []string
	if containerName == name || containerName == "" || isPrimary {
		valuePath = []string{name}
	} else {
		valuePath = []string{name, containerName}
	}

	processProbe := func(p *corev1.Probe, probeName string) error {
		// Initialize as empty object {} per Zero-Default standard
		_ = unstructured.SetNestedField(*values, map[string]interface{}{}, append(valuePath, probeName)...)
		if p != nil {
			if b, err := json.Marshal(p); err == nil {
				var probeMap map[string]interface{}
				if err := json.Unmarshal(b, &probeMap); err == nil {
					cleanMap(probeMap)
					if len(probeMap) > 0 {
						probeYaml, err := yamlformat.Marshal(map[string]interface{}{probeName: probeMap}, 0)
						if err == nil {
							registryKey := probeName + "." + strcase.ToLowerCamel(strings.Join(valuePath, "."))
							helmify.OriginalValuesRegistry.Store(registryKey, strings.TrimSpace(probeYaml))
						}
					}
				}
			}
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
	newEnv := []corev1.EnvVar{}
	for _, e := range c.Env {
		if e.ValueFrom != nil {
			// Keep fieldRef/resourceFieldRef as they can't be in ConfigMaps
			if e.ValueFrom.FieldRef != nil || e.ValueFrom.ResourceFieldRef != nil {
				newEnv = append(newEnv, e)
				continue
			}
			
			// Check for redundant ConfigMap/Secret mappings
			// If it points to the same component's ConfigMap/Secret, it's redundant because of envFrom
			redundant := false
			if e.ValueFrom.ConfigMapKeyRef != nil {
				cmName := processor.ResolveValueName(appMeta, e.ValueFrom.ConfigMapKeyRef.Name)
				if cmName == name {
					redundant = true
				} else {
					e.ValueFrom.ConfigMapKeyRef.Name = processor.TemplatedConfigMapName(appMeta, e.ValueFrom.ConfigMapKeyRef.Name)
				}
			}
			if e.ValueFrom.SecretKeyRef != nil {
				secName := processor.ResolveValueName(appMeta, e.ValueFrom.SecretKeyRef.Name)
				if secName == name {
					redundant = true
				} else {
					e.ValueFrom.SecretKeyRef.Name = processor.TemplatedSecretName(appMeta, e.ValueFrom.SecretKeyRef.Name)
				}
			}
			
			if !redundant {
				newEnv = append(newEnv, e)
			}
			continue
		}
		
		// Move plain value to ConfigMap
		// Use exact key name to preserve casing as requested by user
		err := unstructured.SetNestedField(*values, e.Value, name, "cm", e.Name)
		if err != nil {
			return c, err
		}
	}
	c.Env = newEnv
	return c, nil
}

func AddReloadingAnnotations(appMeta helmify.AppMetadata, objName string, annotations map[string]string, spec *corev1.PodSpec) map[string]string {
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
	annotations["checksum/global-config"] = "[HELMIFY_CHECKSUM_GLOBAL:global-config]"

	for cm := range configMaps {
		cmClean := processor.StripKustomizeHash(cm)
		if strings.Contains(strings.ToLower(cmClean), "global") {
			continue // Already handled by global-config
		}
		comp := processor.NormalizeComponentName(cmClean)
		if comp == "" || comp == "chart" {
			comp = processor.NormalizeComponentName(objName)
		}
		compCamel := strcase.ToLowerCamel(comp)
		compKebab := processor.NormalizeComponentName(comp)

		filename := "cm-" + compKebab + ".yaml"
		if compKebab == processor.NormalizeComponentName(appMeta.ChartName()) {
			filename = "cm.yaml"
		}
		key := "checksum/cm-config"
		if compKebab != processor.NormalizeComponentName(objName) {
			key = "checksum/config-" + compKebab
		}
		annotations[key] = fmt.Sprintf("[HELMIFY_CHECKSUM_CM:%s:cm:%s:%s]", compCamel, key, filename)
	}

	for sec := range secrets {
		secClean := processor.StripKustomizeHash(sec)
		if strings.Contains(strings.ToLower(secClean), "global") {
			continue // Handled by global checksum
		}
		comp := processor.NormalizeComponentName(secClean)
		if comp == "" || comp == "chart" || comp == "secrets" {
			comp = processor.NormalizeComponentName(objName)
		}
		compCamel := strcase.ToLowerCamel(comp)
		compKebab := processor.NormalizeComponentName(comp)

		filename := "secret-" + compKebab + ".yaml"
		if compKebab == processor.NormalizeComponentName(appMeta.ChartName()) {
			filename = "secret.yaml"
		}
		key := "checksum/secret"
		if compKebab != processor.NormalizeComponentName(objName) {
			key = "checksum/secret-" + compKebab
		}
		annotations[key] = fmt.Sprintf("[HELMIFY_CHECKSUM_SECRET:%s:secret:%s:%s]", compCamel, key, filename)
	}

	// Always add checksum for default configmap and secret of the component if not already added
	if objName != "" && objName != "global" && objName != "chart" {
		compKebab := processor.NormalizeComponentName(objName)
		if _, exists := annotations["checksum/cm-config"]; !exists {
			cmFilename := "cm-" + compKebab + ".yaml"
			if compKebab == processor.NormalizeComponentName(appMeta.ChartName()) {
				cmFilename = "cm.yaml"
			}
			annotations["checksum/cm-config"] = fmt.Sprintf("[HELMIFY_CHECKSUM_CM:%s:cm:checksum/cm-config:%s]", objName, cmFilename)
		}
		if _, exists := annotations["checksum/secret"]; !exists {
			secretFilename := "secret-" + compKebab + ".yaml"
			if compKebab == processor.NormalizeComponentName(appMeta.ChartName()) {
				secretFilename = "secret.yaml"
			}
			annotations["checksum/secret"] = fmt.Sprintf("[HELMIFY_CHECKSUM_SECRET:%s:secret:checksum/secret:%s]", objName, secretFilename)
		}
	}

	// Filter out static placeholders from Kustomize
	for k, v := range annotations {
		if v == "$(CONFIG_HASH)" || v == "$(SECRET_HASH)" {
			delete(annotations, k)
		}
	}

	return annotations
}

// ReplacePlaceholders replaces HELMIFY_WITH, HELMIFY_ENV_FROM, grace period, and checksum placeholders with actual Helm templates.
func ReplacePlaceholders(s string, chartName string) string {
	// 1. Handle single quotes: key: '[HELMIFY_WITH:path:indent]'
	r1 := regexp.MustCompile(`(?m)^(\s*)([a-zA-Z0-9]+):\s*'\[HELMIFY_WITH:([^:]+):([0-9]+)\]'`)
	s = r1.ReplaceAllString(s, "{{- with .Values.${3} }}\n${1}${2}:\n${1}  {{- toYaml . | nindent ${4} }}\n${1}{{- end }}")

	// 2. Handle block scalars if they occur: key: |-\n  [HELMIFY_WITH:path:indent]
	r2 := regexp.MustCompile(`(?m)^(\s*)([a-zA-Z0-9]+):\s*\|-\s*\n\s*\[HELMIFY_WITH:([^:]+):([0-9]+)\]`)
	s = r2.ReplaceAllString(s, "{{- with .Values.${3} }}\n${1}${2}:\n${1}  {{- toYaml . | nindent ${4} }}\n${1}{{- end }}")
	// 3. Handle HELMIFY_ENV_FROM: envFrom: '[HELMIFY_ENV_FROM:name:kebabName:indent]'
	r3_3 := regexp.MustCompile(`(?m)^(\s*)envFrom:\s*'\[HELMIFY_ENV_FROM:([^:]+):([^:]+):([0-9]+)\]'`)
	s = r3_3.ReplaceAllString(s, fmt.Sprintf(`${1}envFrom:
${1}{{- if and .Values.global .Values.global.cm (not (empty .Values.global.cm)) }}
${1}- configMapRef:
${1}    name: {{ include "%[1]s.fullname" . }}-global
${1}{{- end }}
${1}{{- if and .Values.global .Values.global.secret (not (empty .Values.global.secret)) }}
${1}- secretRef:
${1}    name: {{ include "%[1]s.fullname" . }}-global
${1}{{- end }}
${1}{{- if (index .Values "${2}").cm }}
${1}- configMapRef:
${1}    name: {{ include "%[1]s.fullname" . }}-${3}-cm
${1}{{- end }}
${1}{{- if (index .Values "${2}").secret }}
${1}- secretRef:
${1}    name: {{ include "%[1]s.fullname" . }}-${3}-secrets
${1}{{- end }}`, chartName))

	// Handle legacy 2-parameter placeholder for tests or non-component deployments
	r3_2 := regexp.MustCompile(`(?m)^(\s*)envFrom:\s*'\[HELMIFY_ENV_FROM:([^:]+):([0-9]+)\]'`)
	s = r3_2.ReplaceAllString(s, fmt.Sprintf(`${1}envFrom:
${1}{{- if and .Values.global .Values.global.cm (not (empty .Values.global.cm)) }}
${1}- configMapRef:
${1}    name: {{ include "%[1]s.fullname" . }}-global
${1}{{- end }}
${1}{{- if and .Values.global .Values.global.secret (not (empty .Values.global.secret)) }}
${1}- secretRef:
${1}    name: {{ include "%[1]s.fullname" . }}-global
${1}{{- end }}
${1}{{- if (index .Values "${2}").cm }}
${1}- configMapRef:
${1}    name: {{ include "%[1]s.fullname" . }}-${2}-cm
${1}{{- end }}
${1}{{- if (index .Values "${2}").secret }}
${1}- secretRef:
${1}    name: {{ include "%[1]s.fullname" . }}-${2}-secrets
${1}{{- end }}`, chartName))

	// 4. Handle global checksum placeholders
	rGlobal := regexp.MustCompile(`(?m)^(\s*)checksum/global-config:\s*'\[HELMIFY_CHECKSUM_GLOBAL:global-config\]'`)
	s = rGlobal.ReplaceAllString(s, `${1}{{- if and .Values.global .Values.global.cm (not (empty .Values.global.cm)) }}
${1}checksum/global-config: {{ include (print $.Template.BasePath "/cm-global.yaml") . | sha256sum }}
${1}{{- end }}
${1}{{- if and .Values.global .Values.global.secret (not (empty .Values.global.secret)) }}
${1}checksum/global-secret: {{ include (print $.Template.BasePath "/secret-global.yaml") . | sha256sum }}
${1}{{- end }}`)

	// 5. Handle CM checksum placeholders
	rCM := regexp.MustCompile(`(?m)^(\s*)([a-zA-Z0-9\-/]+):\s*'\[HELMIFY_CHECKSUM_CM:([^:]+):([^:]+):([^:]+):([^\]]+)\]'`)
	s = rCM.ReplaceAllString(s, `${1}{{- if and (index .Values "${3}") (index .Values "${3}" "${4}") }}
${1}${2}: {{ include (print $.Template.BasePath "/${6}") . | sha256sum }}
${1}{{- end }}`)

	// 6. Handle Secret checksum placeholders
	rSecret := regexp.MustCompile(`(?m)^(\s*)([a-zA-Z0-9\-/]+):\s*'\[HELMIFY_CHECKSUM_SECRET:([^:]+):([^:]+):([^:]+):([^\]]+)\]'`)
	s = rSecret.ReplaceAllString(s, `${1}{{- if and (index .Values "${3}") (index .Values "${3}" "${4}") }}
${1}${2}: {{ include (print $.Template.BasePath "/${6}") . | sha256sum }}
${1}{{- end }}`)

	// 7. Handle terminationGracePeriodSeconds placeholders
	rGrace := regexp.MustCompile(`(?m)^(\s*)terminationGracePeriodSeconds:\s*'\[HELMIFY_GRACE_PERIOD:([^\]]+)\]'`)
	s = rGrace.ReplaceAllString(s, `${1}{{- if not (kindIs "nil" .Values.${2}.terminationGracePeriodSeconds) }}
${1}terminationGracePeriodSeconds: {{ .Values.${2}.terminationGracePeriodSeconds }}
${1}{{- end }}`)

	// 8. Handle PVC volumes conditional placement
	rPVC := regexp.MustCompile(`(?m)^(\s{6})-\s*name:\s*'\[HELMIFY_PVC_VOL:([^:]+):([^\]]+)\]'\n(\s{8})persistentVolumeClaim:\n(\s{10})claimName:\s*([^\n]+)`)
	s = rPVC.ReplaceAllString(s, `${1}{{- if and (index .Values "${2}") (index .Values "${2}" "persistence") (index .Values "${2}" "persistence" "enabled") }}
${1}- name: ${3}
689: ${4}persistentVolumeClaim:
690: ${5}claimName: ${6}
691: ${1}{{- end }}`)

	// 9. Handle PVC volume mounts conditional placement
	rMount := regexp.MustCompile(`(?m)^(\s{8})-\s*mountPath:\s*([^\n]+)\n((\s{10}[a-zA-Z]+:\s*[^\n]+\n)*)\s{10}name:\s*'\[HELMIFY_PVC_MOUNT:([^:]+):([^\]]+)\]'((\n\s{10}[a-zA-Z]+:\s*[^\n]+)*)`)
	s = rMount.ReplaceAllString(s, `${1}{{- if and (index .Values "${5}") (index .Values "${5}" "persistence") (index .Values "${5}" "persistence" "enabled") }}
${1}- mountPath: ${2}
${3}${1}  name: ${6}${7}
${1}{{- end }}`)

	return s
}

func cleanMap(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		if subMap, ok := v.(map[string]interface{}); ok {
			cleanMap(subMap)
			if len(subMap) == 0 {
				delete(m, k)
			}
		}
	}
}

func ResolveConfigMapVolumeName(appMeta helmify.AppMetadata, name string, compName string) string {
	return processor.TemplatedConfigMapName(appMeta, name)
}

func ResolveSecretVolumeName(appMeta helmify.AppMetadata, name string, compName string) string {
	return processor.TemplatedSecretName(appMeta, name)
}
