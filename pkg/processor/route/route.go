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

	name := processor.ObjectValueName(appMeta, obj)
	nameCamel := strcase.ToLowerCamel(processor.GetComponent(obj))

	suffix := "route"
	if obj.GetName() == appMeta.ChartName() {
		suffix = "none"
	} else {
		s := strings.TrimPrefix(obj.GetName(), appMeta.ChartName())
		s = strings.TrimPrefix(s, "-")
		s = strings.TrimPrefix(s, "route-")
		s = strings.TrimPrefix(s, "route")
		if s != "" {
			suffix = s
		}
	}

	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
	if err != nil {
		return true, nil, err
	}

	routeKey := "route"
	if suffix != "none" && suffix != "route" {
		routeKey = "route" + strcase.ToCamel(suffix)
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

	if host, hasHost := spec["host"]; hasHost && host != "" {
		hostStr, ok := host.(string)
		if ok {
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
	if tlsRaw, hasTls := spec["tls"]; hasTls {
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

	resultName := name
	if obj.GetName() == appMeta.ChartName() {
		resultName = ""
	}

	return true, &routeResult{
		name:   resultName,
		data:   data,
		values: values,
	}, nil
}

func replaceSingleQuotes(s string) string {
	re := regexp.MustCompile(`'({{((.*|.*\n.*))}}.*)'`)
	return re.ReplaceAllString(s, "${1}")
}

type routeResult struct {
	name   string
	data   string
	values helmify.Values
}

func (r *routeResult) Filename() string {
	if r.name == "chart" || r.name == "" {
		return "route.yaml"
	}
	name := strings.TrimPrefix(r.name, "route-")
	name = strings.TrimPrefix(name, "route")
	if name == "" {
		return "route.yaml"
	}
	return fmt.Sprintf("route-%s.yaml", name)
}

func (r *routeResult) Values() helmify.Values {
	return r.values
}

func (r *routeResult) Write(writer io.Writer) error {
	_, err := writer.Write([]byte(r.data))
	return err
}
