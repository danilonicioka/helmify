package configmap

import (
	"fmt"
	"io"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/arttor/helmify/pkg/processor"

	"github.com/arttor/helmify/pkg/helmify"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
	"github.com/iancoleman/strcase"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var configMapTempl = template.Must(template.New("configMap").Funcs(sprig.TxtFuncMap()).Parse(
	`{{ .Meta }}
{{- if .Immutable }}
{{ .Immutable }}
{{- end }}
{{- if .BinaryData }}
{{ .BinaryData }}
{{- end }}
data:
{{- if (index .Values .Name).cm }}
{{- range $key, $value := (index .Values .Name).cm }}
  {{ $key }}: {{ $value | quote }}
{{- end }}
{{- end }}
{{- if .Values.global }}
{{- range $key, $value := .Values.global }}
  {{ $key }}: {{ $value | quote }}
{{- end }}
{{- end }}`))

var configMapGVC = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "ConfigMap",
}

// New creates processor for k8s ConfigMap resource.
func New() helmify.Processor {
	return &configMap{}
}

type configMap struct{}

// Process k8s ConfigMap object into template. Returns false if not capable of processing given resource type.
func (d configMap) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != configMapGVC {
		return false, nil, nil
	}
	var immutable, binaryData string
	var err error

	if field, exists, _ := unstructured.NestedBool(obj.Object, "immutable"); exists {
		immutable, err = yamlformat.Marshal(map[string]interface{}{"immutable": field}, 0)
		if err != nil {
			return true, nil, err
		}
	}
	if field, exists, _ := unstructured.NestedStringMap(obj.Object, "binaryData"); exists {
		binaryData, err = yamlformat.Marshal(map[string]interface{}{"binaryData": field}, 0)
		if err != nil {
			return true, nil, err
		}
	}

	valueName := processor.ObjectValueName(appMeta, obj)
	compName := processor.GetComponent(obj)
	var values helmify.Values
	if field, exists, _ := unstructured.NestedStringMap(obj.Object, "data"); exists {
		_, values = parseMapData(field, compName)
		// Add global defaults
		if values["global"] == nil {
			values["global"] = map[string]interface{}{
				"TZ":                        "America/Belem",
				"KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
			}
		}
	}

	suffix := processor.GetDynamicSuffix(appMeta, obj, "cm")
	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
	if err != nil {
		return true, nil, err
	}

	return true, &result{
		name: valueName,
		data: struct {
			Name       string
			Meta       string
			Immutable  string
			BinaryData string
		}{
			Name:       valueName,
			Meta:       meta,
			Immutable:  immutable,
			BinaryData: binaryData,
		},
		values: values,
	}, nil
}

func parseMapData(data map[string]string, configName string) (map[string]string, helmify.Values) {
	values := helmify.Values{}
	for key, value := range data {
		// Use the key exactly as it is to preserve application requirements
		// Only camel-case the configName prefix
		configNameCamel := strcase.ToLowerCamel(configName)
		valuesNamePath := []string{configNameCamel, "cm", key}
		
		err := unstructured.SetNestedField(values, value, valuesNamePath...)
		if err != nil {
			logrus.WithError(err).Errorf("unable to process configmap data: %v", valuesNamePath)
			continue
		}
	}
	return data, values
}

type result struct {
	name string
	data struct {
		Name       string
		Meta       string
		Immutable  string
		BinaryData string
	}
	values helmify.Values
}

func (r *result) Filename() string {
	if r.name == "chart" || r.name == "" {
		return "cm.yaml"
	}
	return fmt.Sprintf("cm-%s.yaml", r.name)
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	nameCamel := strcase.ToLowerCamel(r.name)
	return configMapTempl.Execute(writer, struct {
		Name       string
		Meta       string
		Immutable  string
		BinaryData string
		Values     helmify.Values
	}{
		Name:       nameCamel,
		Meta:       r.data.Meta,
		Immutable:  r.data.Immutable,
		BinaryData: r.data.BinaryData,
		Values:     r.values,
	})
}
