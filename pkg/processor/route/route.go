package route

import (
	"fmt"
	"io"
	"strings"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	"github.com/iancoleman/strcase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var routeGVC = schema.GroupVersionKind{
	Group:   "route.openshift.io",
	Version: "v1",
	Kind:    "Route",
}

// New creates processor for OpenShift Route resource.
func New() helmify.Processor {
	return &route{}
}

type route struct{}

func (r route) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != routeGVC {
		return false, nil, nil
	}

	// === TJPA SPECIFICATION: Skip standalone processing of existing external route manifests ===
	if strings.HasSuffix(obj.GetName(), "-ext") {
		return true, nil, nil
	}

	name := processor.ObjectValueName(appMeta, obj)

	// Extract spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return true, nil, fmt.Errorf("unable to read route spec")
	}

	// Resolve target service name
	toServiceName := name
	if toRaw, hasTo := spec["to"]; hasTo {
		if to, ok := toRaw.(map[string]interface{}); ok {
			if toName, ok := to["name"].(string); ok && toName != "" {
				toServiceName = toName
			}
		}
	}

	// Find the component of the target service
	targetComponent := ""
	serviceNameClean := strings.ToLower(processor.StripKustomizeHash(toServiceName))
	for _, o := range appMeta.Objects() {
		if strings.ToLower(o.GetKind()) == "service" {
			objNameClean := strings.ToLower(processor.StripKustomizeHash(o.GetName()))
			if objNameClean == serviceNameClean {
				targetComponent = processor.GetComponent(o)
				break
			}
		}
	}

	if targetComponent == "" {
		targetComponent = processor.GetComponent(obj)
	}

	nameCamel := strcase.ToLowerCamel(targetComponent)
	if nameCamel == "" {
		nameCamel = strcase.ToLowerCamel(name)
	}

	values := helmify.Values{}

	hostStr := ""
	if host, hasHost := spec["host"]; hasHost && host != "" {
		if h, ok := host.(string); ok {
			hostStr = h
		}
	}
	if hostStr == "" {
		hostStr = fmt.Sprintf("%s.apps.ocp-hub.i.tj.pa.gov.br", name)
	}

	// Capture annotations
	annotationsMap := map[string]interface{}{}
	if len(obj.GetAnnotations()) != 0 {
		for k, v := range obj.GetAnnotations() {
			annotationsMap[k] = v
		}
	}

	// Capture tls
	var tlsVal interface{}
	if tlsRaw, hasTls := spec["tls"]; hasTls {
		tlsVal = tlsRaw
	} else {
		tlsVal = map[string]interface{}{
			"termination":                   "edge",
			"insecureEdgeTerminationPolicy": "Redirect",
		}
	}

	// 3-Route structure for values.yaml
	routeValues := map[string]interface{}{
		"annotations": annotationsMap,
		"tls":         tlsVal,
		"path":        "",
		"default": map[string]interface{}{
			"enabled": true,
			"host":    hostStr,
		},
		"internal": map[string]interface{}{
			"enabled": false,
			"host":    fmt.Sprintf("%s-int.i.tjpa.jus.br", name),
		},
		"external": map[string]interface{}{
			"enabled": false,
			"host":    fmt.Sprintf("%s.tjpa.jus.br", name),
		},
	}

	isPrimary := strings.ToLower(name) == strings.ToLower(targetComponent)
	var valuesPath string
	if isPrimary {
		valuesPath = fmt.Sprintf("%s.route", nameCamel)
		err := unstructured.SetNestedField(values, routeValues, nameCamel, "route")
		if err != nil {
			return true, nil, err
		}
	} else {
		routeNameCamel := strcase.ToLowerCamel(name)
		valuesPath = fmt.Sprintf("%s.routes.%s", nameCamel, routeNameCamel)
		err := unstructured.SetNestedField(values, routeValues, nameCamel, "routes", routeNameCamel)
		if err != nil {
			return true, nil, err
		}
	}

	templatedToService := processor.TemplatedServiceName(appMeta, toServiceName)

	// Resolve target port
	targetPortValue := "http"
	if portRaw, hasPort := spec["port"]; hasPort {
		if port, ok := portRaw.(map[string]interface{}); ok {
			if targetPort, ok := port["targetPort"]; ok {
				if tp, ok := targetPort.(string); ok && tp != "" {
					targetPortValue = tp
				} else if tpInt, ok := targetPort.(int64); ok {
					targetPortValue = fmt.Sprintf("%d", tpInt)
				}
			}
		}
	}

	// Construct route templates matching models/multi/templates/route-*.yaml style but combined using ---
	routeMetadataName := fmt.Sprintf("{{ include \"%s.fullname\" . }}-%s", appMeta.ChartName(), name)
	routeMetadataNameInt := fmt.Sprintf("{{ include \"%s.fullname\" . }}-%s-int", appMeta.ChartName(), name)
	routeMetadataNameExt := fmt.Sprintf("{{ include \"%s.fullname\" . }}-%s-ext", appMeta.ChartName(), name)

	// Component label generation: for multi-deployment projects include component suffix, otherwise just chart fullname.
	var componentLabelVal string
	normalizedComp := processor.NormalizeComponentName(targetComponent)
	if processor.IsMultiDeployment(appMeta) && normalizedComp != "" {
		componentLabelVal = fmt.Sprintf("{{ include \"%s.fullname\" . }}-%s", appMeta.ChartName(), normalizedComp)
	} else {
		componentLabelVal = fmt.Sprintf("{{ include \"%s.fullname\" . }}", appMeta.ChartName())
	}

	// Preserve original logic for metadata names.
	if targetComponent == appMeta.ChartName() {
		if name == targetComponent {
			routeMetadataName = fmt.Sprintf("{{ include \"%s.fullname\" . }}", appMeta.ChartName())
			routeMetadataNameInt = fmt.Sprintf("{{ include \"%s.fullname\" . }}-int", appMeta.ChartName())
			routeMetadataNameExt = fmt.Sprintf("{{ include \"%s.fullname\" . }}-ext", appMeta.ChartName())
		}
	}

	data := fmt.Sprintf(`{{- if .Values.%[1]s -}}

{{- if .Values.%[1]s.default.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: %[3]s
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}

  {{- with .Values.%[1]s.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.default.host }}
  host: {{ .Values.%[1]s.default.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.path }}
  path: {{ .Values.%[1]s.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.tls }}
  tls:
    {{- toYaml .Values.%[1]s.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: %[4]s
    weight: 100
  port:
    targetPort: %[5]s
  wildcardPolicy: None
---
{{- end }}

{{- if .Values.%[1]s.internal.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: %[7]s
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}

  {{- with .Values.%[1]s.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.internal.host }}
  host: {{ .Values.%[1]s.internal.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.path }}
  path: {{ .Values.%[1]s.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.tls }}
  tls:
    {{- toYaml .Values.%[1]s.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: %[4]s
    weight: 100
  port:
    targetPort: %[5]s
  wildcardPolicy: None
---
{{- end }}

{{- if .Values.%[1]s.external.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: %[8]s
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}

  {{- with .Values.%[1]s.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.external.host }}
  host: {{ .Values.%[1]s.external.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.path }}
  path: {{ .Values.%[1]s.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.tls }}
  tls:
    {{- toYaml .Values.%[1]s.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: %[4]s
    weight: 100
  port:
    targetPort: %[5]s
  wildcardPolicy: None
{{- end }}

{{- end }}`, valuesPath, appMeta.ChartName(), routeMetadataName, templatedToService, targetPortValue, componentLabelVal, routeMetadataNameInt, routeMetadataNameExt)

	return true, &routeResult{
		name:   name,
		data:   data,
		values: values,
	}, nil
}

type routeResult struct {
	name   string
	data   string
	values helmify.Values
}

func (r *routeResult) Filename() string {
	return ""
}

func (r *routeResult) Values() helmify.Values {
	return r.values
}

func (r *routeResult) Write(writer io.Writer) error {
	if r.Filename() == "" {
		return nil // Skip writing combined route file as per user request
	}
	_, err := writer.Write([]byte(r.data))
	return err
}

// Ensure Template interface is satisfied
var _ helmify.Template = (*routeResult)(nil)
