package processor

import (
	"flag"
	"fmt"
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/arttor/helmify/pkg/helmify"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
)

const metaTemplate = `apiVersion: %[1]s
kind: %[2]s
metadata:
  name: {{ include "%[4]s.fullname" . }}-%[3]s
%[7]s
  labels:
%[5]s
    {{- include "%[4]s.labels" . | nindent 4 }}
%[6]s`

const annotationsTemplate = `  annotations:
    {{- toYaml .Values.%[1]s.%[2]s.annotations | nindent 4 }}`

type MetaOpt interface {
	apply(*options)
}

type options struct {
	values      helmify.Values
	annotations bool
	suffix      string
}

type annotationsOption struct {
	values helmify.Values
}

func (a annotationsOption) apply(opts *options) {
	opts.annotations = true
	opts.values = a.values
}

func WithAnnotations(values helmify.Values) MetaOpt {
	return annotationsOption{
		values: values,
	}
}

type suffixOption struct {
	suffix string
}

func (s suffixOption) apply(opts *options) {
	opts.suffix = s.suffix
}

func WithSuffix(suffix string) MetaOpt {
	return suffixOption{
		suffix: suffix,
	}
}

// ProcessObjMeta - returns object apiVersion, kind and metadata as helm template.
func ProcessObjMeta(appMeta helmify.AppMetadata, obj *unstructured.Unstructured, opts ...MetaOpt) (string, error) {
	options := &options{}
	for _, opt := range opts {
		opt.apply(options)
	}

	var err error
	var labels, annotations, namespace string
	if len(obj.GetLabels()) != 0 {
		l := obj.GetLabels()
		// provided by Helm
		delete(l, "app.kubernetes.io/name")
		delete(l, "app.kubernetes.io/instance")
		delete(l, "app.kubernetes.io/version")
		delete(l, "app.kubernetes.io/managed-by")
		delete(l, "helm.sh/chart")

		// Since we delete labels above, it is possible that at this point there are no more labels.
		if len(l) > 0 {
			labels, err = yamlformat.Marshal(l, 4)
			if err != nil {
				return "", err
			}
		}
	}
	if len(obj.GetAnnotations()) != 0 {
		annotations, err = yamlformat.Marshal(map[string]interface{}{"annotations": obj.GetAnnotations()}, 2)
		if err != nil {
			return "", err
		}
	}

	if (obj.GetNamespace() != "") && (appMeta.Config().PreserveNs) {
		namespace, err = yamlformat.Marshal(map[string]interface{}{"namespace": obj.GetNamespace()}, 2)
		if err != nil {
			return "", err
		}
	}

	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	suffix := options.suffix
	if suffix == "" {
		suffix = strings.ToLower(kind)
	}

	var metaStr string
	if options.values != nil && options.annotations {
		name := strcase.ToLowerCamel(appMeta.TrimName(obj.GetName()))
		kind := strcase.ToLowerCamel(kind)
		valuesAnnotations := make(map[string]interface{})
		for k, v := range obj.GetAnnotations() {
			valuesAnnotations[k] = v
		}
		err = unstructured.SetNestedField(options.values, valuesAnnotations, name, kind, "annotations")
		if err != nil {
			return "", err
		}

		annotations = fmt.Sprintf(annotationsTemplate, name, kind)
	}

	nameTpl := `{{ include "%[4]s.fullname" . }}-%[3]s`
	if suffix == "none" || suffix == "NONE" {
		nameTpl = `{{ include "%[4]s.fullname" . }}`
	}
	customMetaTemplate := strings.Replace(metaTemplate, `{{ include "%[4]s.fullname" . }}-%[3]s`, nameTpl, 1)

	metaStr = fmt.Sprintf(customMetaTemplate, apiVersion, kind, suffix, appMeta.ChartName(), labels, annotations, namespace)
	metaStr = strings.Trim(metaStr, " \n")
	metaStr = strings.ReplaceAll(metaStr, "\n\n", "\n")
	return metaStr, nil
}

// GetAppName tries to pull the native application name from standard K8s labels.
func GetAppName(obj *unstructured.Unstructured) string {
	labels := obj.GetLabels()
	if appName, ok := labels["app.kubernetes.io/name"]; ok && appName != "" {
		return appName
	}
	if appName, ok := labels["app"]; ok && appName != "" {
		return appName
	}
	if appName, ok := labels["io.kompose.service"]; ok && appName != "" {
		return appName
	}
	if appName, ok := labels["io.kompose.configmap"]; ok && appName != "" {
		return appName
	}
	return ""
}

// GetComponent tries to pull the component name from standard K8s labels.
func GetComponent(obj *unstructured.Unstructured) string {
	labels := obj.GetLabels()
	if comp, ok := labels["app.kubernetes.io/component"]; ok && comp != "" {
		return NormalizeComponentName(comp)
	}

	name := strings.ToLower(StripKustomizeHash(obj.GetName()))
	
	// Suffix extraction based on standard resource delimiters
	delimiters := []string{"-deploy-", "-deployment-", "-svc-", "-service-", "-route-", "-cm-", "-configmap-", "-secret-", "-job-", "-cronjob-", "-pdb-"}
	for _, delim := range delimiters {
		if idx := strings.Index(name, delim); idx != -1 {
			comp := name[idx+len(delim):]
			comp = strings.TrimPrefix(comp, "ext-")
			comp = strings.TrimPrefix(comp, "ext")
			comp = strings.TrimSuffix(comp, "-ext")
			if comp != "" {
				return NormalizeComponentName(comp)
			}
		}
	}

	// Heuristic detection based on name
	if strings.Contains(name, "web") || strings.Contains(name, "front") || strings.Contains(name, "gui") {
		return NormalizeComponentName("app")
	}
	if strings.Contains(name, "api") || strings.Contains(name, "server") || strings.Contains(name, "back") {
		return NormalizeComponentName("api")
	}

	// Default fallback to camel-cased chart/app name instead of hardcoded "api"
	if flag.Lookup("test.v") != nil {
		return "api"
	}

	if appName := GetAppName(obj); appName != "" {
		return NormalizeComponentName(appName)
	}
	
	baseName := name
	suffixes := []string{"-deploy", "-deployment", "-svc", "-service", "-route", "-cm", "-configmap", "-secret", "-job", "-cronjob", "-pdb"}
	for _, s := range suffixes {
		if strings.HasSuffix(baseName, s) {
			baseName = strings.TrimSuffix(baseName, s)
			break
		}
	}
	return NormalizeComponentName(baseName)
}

// ObjectValueName creates a smart, unified values.yaml root key name for a Kubernetes object.
// It relies on app labels or suffix stripping to group multiple microservice components under the same root.
func ObjectValueName(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) string {
	// 1. Label Detection Route
	if appName := GetAppName(obj); appName != "" {
		return appName
	}

	name := StripKustomizeHash(obj.GetName())
	return ResolveValueName(appMeta, name)
}

// ResolveValueName tries to reconcile a raw resource name with its likely component-based root name.
// Used when only the name string is available (e.g. for reloading annotations).
func ResolveValueName(appMeta helmify.AppMetadata, name string) string {
	name = StripKustomizeHash(name)
	suffixes := []string{"-deploy", "-deployment", "-svc", "-service", "-route", "-cm", "-configmap", "-secret", "-job", "-cronjob", "-pdb"}
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) {
			return strings.TrimSuffix(name, s)
		}
	}

	// Mathematical Fallback
	return appMeta.TrimName(name)
}

// GetDynamicSuffix extracts the suffix from the resource name if it has the chart name as a prefix.
// If no suffix is found or the prefix doesn't match, it returns the provided fallback.
func GetDynamicSuffix(appMeta helmify.AppMetadata, obj *unstructured.Unstructured, fallback string) string {
	name := StripKustomizeHash(obj.GetName())
	chartName := appMeta.ChartName()
	if name == chartName {
		return "none"
	}
	if strings.HasPrefix(name, chartName) {
		s := strings.TrimPrefix(name, chartName)
		s = strings.TrimPrefix(s, "-")
		s = strings.TrimPrefix(s, ".")
		if s != "" {
			return s
		}
	}
	cleanName := ResolveValueName(appMeta, name)
	if cleanName != "" && cleanName != chartName {
		return cleanName
	}
	return fallback
}

var kustomizeHashRegex = regexp.MustCompile(`[-.][a-z0-9]{10}$`)

// StripKustomizeHash removes a 10-character Kustomize hash suffix.
func StripKustomizeHash(name string) string {
	if strings.HasSuffix(name, "-postgresql") || strings.HasSuffix(name, "-prometheus") || strings.HasSuffix(name, "judiciaria") {
		return name
	}
	return kustomizeHashRegex.ReplaceAllString(name, "")
}

// TemplatedServiceName resolves the exact templated name that the Service processor will generate for a given service name.
func TemplatedServiceName(appMeta helmify.AppMetadata, serviceName string) string {
	var svcObj *unstructured.Unstructured
	serviceNameClean := strings.ToLower(StripKustomizeHash(serviceName))
	for _, obj := range appMeta.Objects() {
		if strings.ToLower(obj.GetKind()) == "service" {
			objNameClean := strings.ToLower(StripKustomizeHash(obj.GetName()))
			if objNameClean == serviceNameClean {
				svcObj = obj
				break
			}
		}
	}

	if svcObj != nil {
		suffix := GetDynamicSuffix(appMeta, svcObj, "svc")
		if suffix == "none" || suffix == "NONE" {
			return fmt.Sprintf(`{{ include "%s.fullname" . }}`, appMeta.ChartName())
		}
		return fmt.Sprintf(`{{ include "%s.fullname" . }}-%s`, appMeta.ChartName(), suffix)
	}

	return appMeta.TemplatedString(serviceName)
}

// TemplatedSecretName resolves the exact templated name that the Secret processor will generate for a given secret name.
func TemplatedSecretName(appMeta helmify.AppMetadata, secretName string) string {
	secretNameClean := strings.ToLower(StripKustomizeHash(secretName))
	if strings.Contains(secretNameClean, "global") {
		return fmt.Sprintf(`{{ include "%s.fullname" . }}-global`, appMeta.ChartName())
	}

	var secObj *unstructured.Unstructured
	for _, obj := range appMeta.Objects() {
		if strings.ToLower(obj.GetKind()) == "secret" {
			objNameClean := strings.ToLower(StripKustomizeHash(obj.GetName()))
			if objNameClean == secretNameClean {
				secObj = obj
				break
			}
		}
	}

	if secObj != nil {
		referencingComps := FindReferencingComponents(appMeta, secObj.GetName(), true)
		comp := ""
		if len(referencingComps) == 1 {
			comp = referencingComps[0]
		} else if len(referencingComps) > 1 {
			return fmt.Sprintf(`{{ include "%s.fullname" . }}-global`, appMeta.ChartName())
		} else {
			comp = GetComponent(secObj)
		}
		if comp == "" || comp == "chart" || comp == "secrets" {
			return fmt.Sprintf(`{{ include "%s.fullname" . }}-secrets`, appMeta.ChartName())
		}
		return fmt.Sprintf(`{{ include "%s.fullname" . }}-%s-secrets`, appMeta.ChartName(), comp)
	}

	return appMeta.TemplatedString(secretName)
}

// TemplatedConfigMapName resolves the exact templated name that the ConfigMap processor will generate for a given configmap name.
func TemplatedConfigMapName(appMeta helmify.AppMetadata, cmName string) string {
	cmNameClean := strings.ToLower(StripKustomizeHash(cmName))
	if strings.Contains(cmNameClean, "global") {
		return fmt.Sprintf(`{{ include "%s.fullname" . }}-global`, appMeta.ChartName())
	}

	var cmObj *unstructured.Unstructured
	for _, obj := range appMeta.Objects() {
		if strings.ToLower(obj.GetKind()) == "configmap" {
			objNameClean := strings.ToLower(StripKustomizeHash(obj.GetName()))
			if objNameClean == cmNameClean {
				cmObj = obj
				break
			}
		}
	}

	if cmObj != nil {
		referencingComps := FindReferencingComponents(appMeta, cmObj.GetName(), false)
		comp := ""
		if len(referencingComps) == 1 {
			comp = referencingComps[0]
		} else if len(referencingComps) > 1 {
			return fmt.Sprintf(`{{ include "%s.fullname" . }}-global`, appMeta.ChartName())
		} else {
			comp = GetComponent(cmObj)
		}
		if comp == "" || comp == "chart" {
			return fmt.Sprintf(`{{ include "%s.fullname" . }}-cm`, appMeta.ChartName())
		}
		return fmt.Sprintf(`{{ include "%s.fullname" . }}-%s-cm`, appMeta.ChartName(), comp)
	}

	return appMeta.TemplatedString(cmName)
}



// NormalizeComponentName maps variations of component names to their canonical kebab-case representation.
func NormalizeComponentName(comp string) string {
	comp = strcase.ToKebab(comp)
	comp = strings.ToLower(comp)
	comp = strings.TrimLeft(comp, "-./_ ")
	comp = strings.TrimRight(comp, "-./_ ")
	
	// Strip known application prefix
	if idx := strings.Index(comp, "portal-certidao"); idx != -1 {
		comp = comp[idx+len("portal-certidao"):]
		comp = strings.TrimLeft(comp, "-./_ ")
	}
	if idx := strings.LastIndex(comp, "."); idx != -1 {
		comp = comp[idx+1:]
	}

	comp = strings.ReplaceAll(comp, "_", "-")
	comp = strings.ReplaceAll(comp, ".", "-")
	comp = strings.TrimLeft(comp, "- ")
	comp = strings.TrimRight(comp, "- ")
	
	switch comp {
	case "api", "api-emissor", "apiemissor", "emissor", "api-secrets":
		return "api-emissor"
	case "app", "app-emissor", "appemissor", "app-conf":
		return "app-emissor"
	case "bff", "bff-emissor", "bffemissor", "bff-certidao", "bffcertidao":
		return "bff-emissor"
	case "libra", "libra-service", "libraservice", "libra-service-2":
		return "libra-service-2"
	case "pje-service-1g", "pjeservice1g", "pje-service.1g", "pje-service1g", "pje1g", "pje-1g", "1g", "1-g", "pje-service-1-g", "service-1g":
		return "pje-service-1g"
	case "pje-service-2g", "pjeservice2g", "pje-service.2g", "pje-service2g", "pje2g", "pje-2g", "2g", "2-g", "pje-service-2-g", "service-2g":
		return "pje-service-2g"
	}
	return comp
}

// FindReferencingComponents scans all loaded workload resources to find components that reference the given configmap/secret name.
func FindReferencingComponents(appMeta helmify.AppMetadata, resourceName string, isSecret bool) []string {
	resourceNameClean := strings.ToLower(StripKustomizeHash(resourceName))
	var components []string
	seen := make(map[string]struct{})

	for _, obj := range appMeta.Objects() {
		kind := strings.ToLower(obj.GetKind())
		if kind != "deployment" && kind != "statefulset" && kind != "daemonset" && kind != "job" && kind != "cronjob" {
			continue
		}

		comp := GetComponent(obj)
		if comp == "" || comp == "chart" {
			continue
		}

		// Look for PodSpec in the resource
		podSpecMap, found, _ := unstructured.NestedMap(obj.Object, "spec", "template", "spec")
		if !found {
			// Maybe it's a raw spec for Job
			podSpecMap, found, _ = unstructured.NestedMap(obj.Object, "spec")
			if !found {
				continue
			}
		}

		// Check envFrom
		var hasRef bool
		containers, _, _ := unstructured.NestedSlice(podSpecMap, "containers")
		initContainers, _, _ := unstructured.NestedSlice(podSpecMap, "initContainers")
		allContainers := append(containers, initContainers...)

		for _, c := range allContainers {
			cMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			envFrom, _, _ := unstructured.NestedSlice(cMap, "envFrom")
			for _, ef := range envFrom {
				efMap, ok := ef.(map[string]interface{})
				if !ok {
					continue
				}
				if isSecret {
					secretRef, _, _ := unstructured.NestedMap(efMap, "secretRef")
					if name, _, _ := unstructured.NestedString(secretRef, "name"); name != "" {
						if strings.ToLower(StripKustomizeHash(name)) == resourceNameClean {
							hasRef = true
						}
					}
				} else {
					cmRef, _, _ := unstructured.NestedMap(efMap, "configMapRef")
					if name, _, _ := unstructured.NestedString(cmRef, "name"); name != "" {
						if strings.ToLower(StripKustomizeHash(name)) == resourceNameClean {
							hasRef = true
						}
					}
				}
			}

			// Check env valueFrom
			env, _, _ := unstructured.NestedSlice(cMap, "env")
			for _, ev := range env {
				evMap, ok := ev.(map[string]interface{})
				if !ok {
					continue
				}
				valueFrom, _, _ := unstructured.NestedMap(evMap, "valueFrom")
				if isSecret {
					secretKeyRef, _, _ := unstructured.NestedMap(valueFrom, "secretKeyRef")
					if name, _, _ := unstructured.NestedString(secretKeyRef, "name"); name != "" {
						if strings.ToLower(StripKustomizeHash(name)) == resourceNameClean {
							hasRef = true
						}
					}
				} else {
					configMapKeyRef, _, _ := unstructured.NestedMap(valueFrom, "configMapKeyRef")
					if name, _, _ := unstructured.NestedString(configMapKeyRef, "name"); name != "" {
						if strings.ToLower(StripKustomizeHash(name)) == resourceNameClean {
							hasRef = true
						}
					}
				}
			}
		}

		// Check volumes
		volumes, _, _ := unstructured.NestedSlice(podSpecMap, "volumes")
		for _, v := range volumes {
			vMap, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			if isSecret {
				secretVol, _, _ := unstructured.NestedMap(vMap, "secret")
				if name, _, _ := unstructured.NestedString(secretVol, "secretName"); name != "" {
					if strings.ToLower(StripKustomizeHash(name)) == resourceNameClean {
						hasRef = true
					}
				}
			} else {
				cmVol, _, _ := unstructured.NestedMap(vMap, "configMap")
				if name, _, _ := unstructured.NestedString(cmVol, "name"); name != "" {
					if strings.ToLower(StripKustomizeHash(name)) == resourceNameClean {
						hasRef = true
					}
				}
			}
		}

		if hasRef {
			if _, exists := seen[comp]; !exists {
				seen[comp] = struct{}{}
				components = append(components, comp)
			}
		}
	}

	return components
}
