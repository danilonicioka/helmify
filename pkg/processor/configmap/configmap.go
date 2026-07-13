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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var configMapTempl = template.Must(template.New("configMap").Funcs(sprig.TxtFuncMap()).Parse(
	`{{- if .IsGlobal -}}
{{- if and .Values.global .Values.global.cm -}}
{{ .Meta }}
{{- if .Immutable }}
{{ .Immutable }}
{{- end }}
{{- if .BinaryData }}
{{ .BinaryData }}
{{- end }}
data:
{{- range $key, $value := .Values.global.cm }}
  {{ $key }}: {{ $value | quote }}
{{- end }}
{{- end }}
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

// NewCreates processor for k8s ConfigMap resource.
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

	referencingComps := processor.FindReferencingComponents(appMeta, obj.GetName(), false)

	nameLower := strings.ToLower(obj.GetName())
	isGlobal := false
	if strings.HasSuffix(nameLower, "global") || strings.HasSuffix(nameLower, "global-cm") || strings.HasSuffix(nameLower, "cm-global") || len(referencingComps) > 1 {
		isGlobal = true
	}

	if isGlobal {
		globalValues := map[string]interface{}{}
		if field, exists, _ := unstructured.NestedStringMap(obj.Object, "data"); exists {
			for k, v := range field {
				globalValues[k] = v
			}
		}
		values := helmify.Values{
			"global": map[string]interface{}{
				"cm": globalValues,
			},
		}

		meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix("global"))
		if err != nil {
			return true, nil, err
		}

		return true, &configMapTemplate{
			compName:   "global",
			meta:       meta,
			isGlobal:   true,
			values:     values,
			immutable:  immutable,
			binaryData: binaryData,
		}, nil
	}

	// Non-global configmap processing
	if len(referencingComps) == 0 {
		compName := processor.GetComponent(obj)
		if compName != "" && compName != "chart" {
			referencingComps = []string{compName}
		}
	}

	if len(referencingComps) == 0 {
		return true, nil, nil
	}

	var dataKeys []string
	field, exists, _ := unstructured.NestedStringMap(obj.Object, "data")
	if exists {
		for k := range field {
			dataKeys = append(dataKeys, k)
		}
	}

	var templates []helmify.Template
	for _, comp := range referencingComps {
		compCamel := strcase.ToLowerCamel(comp)
		values := helmify.Values{}
		if exists {
			for key, val := range field {
				valuesNamePath := []string{compCamel, "cm", key}
				_ = unstructured.SetNestedField(values, val, valuesNamePath...)
			}
		}

		suffix := comp + "-cm"
		meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
		if err != nil {
			return true, nil, err
		}

		templates = append(templates, &configMapTemplate{
			compName:      comp,
			chartName:     appMeta.ChartName(),
			nameCamelCase: compCamel,
			meta:          meta,
			values:        values,
			isGlobal:      false,
			dataKeys:      dataKeys,
			immutable:     immutable,
			binaryData:    binaryData,
		})
	}

	return true, &multiTemplate{templates: templates}, nil
}

type configMapTemplate struct {
	compName      string
	chartName     string
	nameCamelCase string
	meta          string
	values        helmify.Values
	isGlobal      bool
	dataKeys      []string
	immutable     string
	binaryData    string
}

func (c *configMapTemplate) Filename() string {
	if c.isGlobal {
		return "cm-global.yaml"
	}
	if c.compName == c.chartName {
		return "cm.yaml"
	}
	return fmt.Sprintf("cm-%s.yaml", c.compName)
}

func (c *configMapTemplate) Values() helmify.Values {
	return c.values
}

func (c *configMapTemplate) Write(writer io.Writer) error {
	return configMapTempl.Execute(writer, struct {
		Name       string
		Meta       string
		Values     helmify.Values
		IsGlobal   bool
		DataKeys   []string
		Immutable  string
		BinaryData string
	}{
		Name:       c.nameCamelCase,
		Meta:       c.meta,
		Values:     c.values,
		IsGlobal:   c.isGlobal,
		DataKeys:   c.dataKeys,
		Immutable:  c.immutable,
		BinaryData: c.binaryData,
	})
}

type multiTemplate struct {
	templates []helmify.Template
}

func (m *multiTemplate) Templates() []helmify.Template {
	return m.templates
}

func (m *multiTemplate) Filename() string {
	return ""
}

func (m *multiTemplate) Values() helmify.Values {
	merged := helmify.Values{}
	for _, t := range m.templates {
		_ = merged.Merge(t.Values())
	}
	return merged
}

func (m *multiTemplate) Write(writer io.Writer) error {
	return nil
}
