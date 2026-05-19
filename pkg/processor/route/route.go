package route

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
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
	nameCamel := strcase.ToLowerCamel(processor.GetComponent(obj))

	rawSuffix := "route"
	if name != appMeta.ChartName() {
		s := strings.TrimPrefix(name, appMeta.ChartName())
		s = strings.TrimPrefix(s, "-")
		s = strings.TrimPrefix(s, "route-")
		s = strings.TrimPrefix(s, "route")
		if s != "" {
			rawSuffix = s
		}
	}

	metadataSuffix := "route"
	if rawSuffix != "route" {
		metadataSuffix = "route-" + rawSuffix
	}

	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(metadataSuffix))
	if err != nil {
		return true, nil, err
	}

	routeKey := "route"
	if rawSuffix != "route" {
		routeKey = "route" + strcase.ToCamel(rawSuffix)
	}

	values := helmify.Values{}
	_, err = values.Add(true, nameCamel, routeKey, "enabled")
	if err != nil {
		return true, nil, err
	}

	// Extract spec
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return true, nil, fmt.Errorf("unable to read route spec")
	}

	var rawHost string
	if host, hasHost := spec["host"]; hasHost && host != "" {
		hostStr, ok := host.(string)
		if ok {
			rawHost = hostStr
			hostTpl, err := values.Add(hostStr, nameCamel, routeKey, "host")
			if err != nil {
				return true, nil, err
			}
			spec["host"] = hostTpl
		}
	}

	if toRaw, hasTo := spec["to"]; hasTo {
		if to, ok := toRaw.(map[string]interface{}); ok {
			if toName, ok := to["name"].(string); ok && toName != "" {
				// Typically, it points to a service in the same app.
				to["name"] = appMeta.TemplatedString(toName)
			}
		}
	}

	if portRaw, hasPort := spec["port"]; hasPort {
		if port, ok := portRaw.(map[string]interface{}); ok {
			if targetPort, ok := port["targetPort"]; ok {
				portTpl, err := values.Add(targetPort, nameCamel, routeKey, "targetPort")
				if err != nil {
					return true, nil, err
				}
				port["targetPort"] = portTpl
			}
		}
	}

	tlsTplStr := ""
	var originalHasTls bool
	if tlsRaw, hasTls := spec["tls"]; hasTls {
		originalHasTls = true
		delete(spec, "tls")
		err := unstructured.SetNestedField(values, tlsRaw, nameCamel, routeKey, "tls")
		if err != nil {
			return true, nil, err
		}
		tlsTplStr = fmt.Sprintf("\n  {{- if .Values.%s.%s.tls }}\n  tls:\n    {{- toYaml .Values.%s.%s.tls | nindent 4 }}\n  {{- end }}", nameCamel, routeKey, nameCamel, routeKey)
	}

	// Output spec
	specYaml, err := yamlformat.Marshal(map[string]interface{}{"spec": spec}, 0)
	if err != nil {
		return true, nil, err
	}
	specStr := replaceSingleQuotes(specYaml)

	data := meta + "\n" + specStr + tlsTplStr
	data = fmt.Sprintf("{{- if .Values.%s.%s.enabled -}}\n%s\n{{- end }}", nameCamel, routeKey, data)

	// === TJPA SPECIFICATION: Route Extension (.apps.oc* to .tjpa.jus.br) ===
	var extTemplate helmify.Template
	if rawHost != "" {
		if idx := strings.Index(rawHost, ".apps.oc"); idx != -1 {
			extHost := rawHost[:idx] + ".tjpa.jus.br"
			routeExtKey := "routeExt"
				if rawSuffix != "route" {
					routeExtKey = "routeExt" + strcase.ToCamel(rawSuffix)
				}

				extValues := helmify.Values{}
				_, err = extValues.Add(false, nameCamel, routeExtKey, "enabled")
				if err != nil {
					return true, nil, err
				}
				extHostTpl, err := extValues.Add(extHost, nameCamel, routeExtKey, "host")
				if err != nil {
					return true, nil, err
				}

				extFilename := "route-ext.yaml"
				if rawSuffix != "route" {
					extFilename = fmt.Sprintf("route-ext-%s.yaml", rawSuffix)
				}

				extMetaSuffix := "route-ext"
				if rawSuffix != "route" {
					extMetaSuffix = "route-ext-" + rawSuffix
				}

				// Re-process metadata with route-ext suffix
				extMeta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(extMetaSuffix))
				if err != nil {
					return true, nil, err
				}

				var annotationsStr string
				if _, hasAnnotations := obj.Object["metadata"].(map[string]interface{})["annotations"]; hasAnnotations {
					annotationsStr = fmt.Sprintf("\n  {{- with .Values.%s.%s.annotations }}\n  annotations:\n    {{- toYaml . | nindent 4 }}\n  {{- end }}", nameCamel, routeKey)
				}

				var tlsStr string
				if originalHasTls {
					tlsStr = fmt.Sprintf("\n  {{- with .Values.%s.%s.tls }}\n  tls:\n    {{- toYaml . | nindent 4 }}\n  {{- end }}", nameCamel, routeKey)
				}

				extData := fmt.Sprintf(`{{- if .Values.%s.%s.enabled -}}
%s
spec:
  host: %s
  port:
    targetPort: http%s%s
  to:
    kind: Service
    name: {{ include "%s.fullname" . }}-svc
{{- end }}`, nameCamel, routeExtKey, extMeta, extHostTpl, annotationsStr, tlsStr, appMeta.ChartName())

				extTemplate = &routeExtResult{
					filename: extFilename,
					data:     extData,
					values:   extValues,
				}
			}
		}

	resultName := ""
	if rawSuffix != "route" {
		resultName = rawSuffix
	}

	return true, &routeResult{
		name:   resultName,
		data:   data,
		values: values,
		ext:    extTemplate,
	}, nil
}

func replaceSingleQuotes(s string) string {
	re := regexp.MustCompile(`'({{((.*|.*\n.*))}}.*)'`)
	return re.ReplaceAllString(s, "${1}")
}

// === TJPA SPECIFICATION: Route Extension (.apps.oc* to .tjpa.jus.br) ===
type routeExtResult struct {
	filename string
	data     string
	values   helmify.Values
}

func (r *routeExtResult) Filename() string {
	return r.filename
}

func (r *routeExtResult) Values() helmify.Values {
	return r.values
}

func (r *routeExtResult) Write(writer io.Writer) error {
	_, err := writer.Write([]byte(r.data))
	return err
}

type routeResult struct {
	name   string
	data   string
	values helmify.Values
	ext    helmify.Template
}

func (r *routeResult) Templates() []helmify.Template {
	if r.ext != nil {
		return []helmify.Template{r, r.ext}
	}
	return []helmify.Template{r}
}

func (r *routeResult) Filename() string {
	if r.name == "chart" || r.name == "" {
		return "route.yaml"
	}
	return fmt.Sprintf("route-%s.yaml", r.name)
}

func (r *routeResult) Values() helmify.Values {
	return r.values
}

func (r *routeResult) Write(writer io.Writer) error {
	_, err := writer.Write([]byte(r.data))
	return err
}
