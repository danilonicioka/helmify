package configmap

import (
	"fmt"
	"io"
	"text/template"

	"github.com/arttor/helmify/pkg/processor"

	"github.com/arttor/helmify/pkg/helmify"
	yamlformat "github.com/arttor/helmify/pkg/yaml"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var configMapTempl, _ = template.New("configMap").Parse(
	`{{ .Meta }}
{{- if .Immutable }}
{{ .Immutable }}
{{- end }}
{{- if .BinaryData }}
{{ .BinaryData }}
{{- end }}
data:
{{- range $key, $value := (index .Values .Name).cm }}
  {{ $key }}: {{ $value | quote }}
{{- end }}
  TZ: {{ .Values.global.timezone | default "America/Belem" | quote }}`)

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
	var values helmify.Values
	if field, exists, _ := unstructured.NestedStringMap(obj.Object, "data"); exists {
		_, values = parseMapData(field, valueName)
		// Restructure values to be under component.cm
		cmValues := values[valueName]
		values = helmify.Values{
			valueName: map[string]interface{}{
				"cm": cmValues,
			},
			"global": map[string]interface{}{
				"timezone":                "America/Belem",
				"kubernetesClusterDomain": "cluster.local",
			},
		}
	}

	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix("cm"))
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
		valuesNamePath := []string{configName, key}
		// handle plain string (we don't need properties parsing for the range strategy)
		templatedVal, err := values.Add(value, valuesNamePath...)
		if err != nil {
			logrus.WithError(err).Errorf("unable to process configmap data: %v", valuesNamePath)
			continue
		}
		data[key] = templatedVal
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
	return configMapTempl.Execute(writer, r.data)
}
