package configmap

import (
	"fmt"
	"io"
	"strings"
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

var configMapFuncMap = func() template.FuncMap {
	f := sprig.TxtFuncMap()
	f["tpl"] = func(string, interface{}) string { return "" }
	return f
}()

var configMapTempl = template.Must(template.New("configMap").Funcs(configMapFuncMap).Parse(
	`{{- if .IsGlobal -}}
{{ .Meta }}
{{- if .Immutable }}
{{ .Immutable }}
{{- end }}
{{- if .BinaryData }}
{{ .BinaryData }}
{{- end }}
data:
{{- if .Values.global }}
{{- range $key, $value := .Values.global }}
  {{ $key }}: {{ $value | quote }}
{{- end }}
{{- end }}
{{- else if .IsCustom -}}
{{ "{" }}{{ "{" }}- if and .Values.{{ .Name }} .Values.{{ .Name }}.{{ .Suffix }} {{ "}" }}{{ "}" }}
{{ .Meta }}
{{- if .Immutable }}
{{ .Immutable }}
{{- end }}
{{- if .BinaryData }}
{{ .BinaryData }}
{{- end }}
data:
{{- range $key := .DataKeys }}
  {{ if eq $key "nginx" }}nginx.conf{{ else }}{{ $key }}{{ end }}: |
{{ "    " }}{{ "{{- tpl .Values." }}{{ $.Name }}{{ "." }}{{ $.Suffix }}{{ " . | nindent 4 }}" }}
{{- end }}
{{ "{" }}{{ "{" }}- end {{ "}" }}{{ "}" }}
{{- else -}}
{{ "{" }}{{ "{" }}- if and .Values.{{ .Name }} .Values.{{ .Name }}.cm {{ "}" }}{{ "}" }}
{{ .Meta }}
{{- if .Immutable }}
{{ .Immutable }}
{{- end }}
{{- if .BinaryData }}
{{ .BinaryData }}
{{- end }}
data:
{{ "{" }}{{ "{" }}- range $key, $val := .Values.{{ .Name }}.cm {{ "}" }}{{ "}" }}
  {{ "{{ $key }}" }}: {{ "{{ $val | quote }}" }}
{{ "{" }}{{ "{" }}- end {{ "}" }}{{ "}" }}
{{ "{" }}{{ "{" }}- end {{ "}" }}{{ "}" }}
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
	isGlobal := false
	nameLower := strings.ToLower(obj.GetName())
	if strings.HasSuffix(nameLower, "global") || strings.HasSuffix(nameLower, "global-cm") || strings.HasSuffix(nameLower, "cm-global") {
		isGlobal = true
	}

	suffix := processor.GetDynamicSuffix(appMeta, obj, "cm")
	isCustom := false
	if !isGlobal && suffix != "cm" && suffix != "" {
		isCustom = true
		suffixLower := strings.ToLower(suffix)
		compNameLower := strings.ToLower(compName)
		if strings.HasSuffix(compNameLower, suffixLower) {
			compName = compName[:len(compName)-len(suffixLower)]
		}
		compNameLower = strings.ToLower(compName)
		if strings.HasSuffix(compNameLower, "-") {
			compName = compName[:len(compName)-1]
		}
	}

	var dataKeys []string
	if field, exists, _ := unstructured.NestedStringMap(obj.Object, "data"); exists {
		for k := range field {
			dataKeys = append(dataKeys, k)
		}

		if isCustom {
			values = helmify.Values{}
			compNameCamel := strcase.ToLowerCamel(compName)
			for _, val := range field {
				err := unstructured.SetNestedField(values, val, compNameCamel, suffix)
				if err != nil {
					logrus.WithError(err).Errorf("unable to process custom configmap data")
				}
			}
		} else {
			_, values = parseMapData(field, compName)
		}
	}

	if values == nil {
		values = helmify.Values{}
	}
	if isGlobal {
		if values["global"] == nil {
			values["global"] = map[string]interface{}{
				"TZ":                        "America/Belem",
				"KUBERNETES_CLUSTER_DOMAIN": "cluster.local",
			}
		}
	}

	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
	if err != nil {
		return true, nil, err
	}

	return true, &result{
		name:     valueName,
		compName: strcase.ToLowerCamel(compName),
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
		values:   values,
		isGlobal: isGlobal,
		isCustom: isCustom,
		suffix:   suffix,
		dataKeys: dataKeys,
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
	name     string
	compName string
	data     struct {
		Name       string
		Meta       string
		Immutable  string
		BinaryData string
	}
	values   helmify.Values
	isGlobal bool
	isCustom bool
	suffix   string
	dataKeys []string
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
	nameCamel := r.compName
	if nameCamel == "" {
		nameCamel = "chart"
	}
	return configMapTempl.Execute(writer, struct {
		Name       string
		Meta       string
		Immutable  string
		BinaryData string
		Values     helmify.Values
		IsGlobal   bool
		IsCustom   bool
		Suffix     string
		DataKeys   []string
	}{
		Name:       nameCamel,
		Meta:       r.data.Meta,
		Immutable:  r.data.Immutable,
		BinaryData: r.data.BinaryData,
		Values:     r.values,
		IsGlobal:   r.isGlobal,
		IsCustom:   r.isCustom,
		Suffix:     r.suffix,
		DataKeys:   r.dataKeys,
	})
}
