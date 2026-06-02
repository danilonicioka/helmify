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
	compName := processor.GetComponent(obj)
	nameCamel := strcase.ToLowerCamel(compName)
	if nameCamel == "" {
		nameCamel = strcase.ToLowerCamel(name)
	}

	values := helmify.Values{}

	// Extract spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return true, nil, fmt.Errorf("unable to read route spec")
	}

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

	err := unstructured.SetNestedField(values, routeValues, nameCamel, "route")
	if err != nil {
		return true, nil, err
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
	templatedToService := appMeta.TemplatedString(toServiceName)

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
	data := fmt.Sprintf(`{{- if and .Values.%[1]s .Values.%[1]s.route -}}

{{- if .Values.%[1]s.route.default.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "%[2]s.fullname" . }}-%[3]s
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
    app.kubernetes.io/component: %[3]s
  {{- with .Values.%[1]s.route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.route.default.host }}
  host: {{ .Values.%[1]s.route.default.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.path }}
  path: {{ .Values.%[1]s.route.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.tls }}
  tls:
    {{- toYaml .Values.%[1]s.route.tls | nindent 4 }}
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

{{- if .Values.%[1]s.route.internal.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "%[2]s.fullname" . }}-%[3]s-int
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
    app.kubernetes.io/component: %[3]s
  {{- with .Values.%[1]s.route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.route.internal.host }}
  host: {{ .Values.%[1]s.route.internal.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.path }}
  path: {{ .Values.%[1]s.route.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.tls }}
  tls:
    {{- toYaml .Values.%[1]s.route.tls | nindent 4 }}
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

{{- if .Values.%[1]s.route.external.enabled }}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "%[2]s.fullname" . }}-%[3]s-ext
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
    app.kubernetes.io/component: %[3]s
  {{- with .Values.%[1]s.route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.route.external.host }}
  host: {{ .Values.%[1]s.route.external.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.path }}
  path: {{ .Values.%[1]s.route.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.tls }}
  tls:
    {{- toYaml .Values.%[1]s.route.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: %[4]s
    weight: 100
  port:
    targetPort: %[5]s
  wildcardPolicy: None
{{- end }}

{{- end }}`, nameCamel, appMeta.ChartName(), name, templatedToService, targetPortValue)

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
	return fmt.Sprintf("%s-route.yaml", r.name)
}

func (r *routeResult) Values() helmify.Values {
	return r.values
}

func (r *routeResult) Write(writer io.Writer) error {
	_, err := writer.Write([]byte(r.data))
	return err
}

// Ensure Template interface is satisfied
var _ helmify.Template = (*routeResult)(nil)
