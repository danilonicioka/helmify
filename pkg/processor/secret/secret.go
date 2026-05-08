package secret

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/arttor/helmify/pkg/processor"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var secretTempl, _ = template.New("secret").Parse(
	`{{ .Meta }}
{{- if .Type }}
{{ .Type }}
{{- end }}
data:
{{- range $key, $value := (index .Values .Name).secret }}
  {{ $key }}: {{ $value | b64enc | quote }}
{{- end }}`)

var secretGVC = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "Secret",
}

// New creates processor for k8s Secret resource.
func New() helmify.Processor {
	return &secret{}
}

type secret struct{}

// Process k8s Secret object into template. Returns false if not capable of processing given resource type.
func (d secret) Process(appMeta helmify.AppMetadata, obj *unstructured.Unstructured) (bool, helmify.Template, error) {
	if obj.GroupVersionKind() != secretGVC {
		return false, nil, nil
	}
	sec := corev1.Secret{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &sec)
	if err != nil {
		return true, nil, fmt.Errorf("%w: unable to cast to secret", err)
	}

	valueName := processor.ObjectValueName(appMeta, obj)
	nameCamelCase := strcase.ToLowerCamel(valueName)

	values := helmify.Values{}
	secValues := map[string]interface{}{}
	for key, value := range sec.Data {
		keyCamelCase := strcase.ToLowerCamel(key)
		if key == strings.ToUpper(key) {
			keyCamelCase = strcase.ToLowerCamel(strings.ToLower(key))
		}
		secValues[key] = string(value)
	}
	for key, value := range sec.StringData {
		keyCamelCase := strcase.ToLowerCamel(key)
		if key == strings.ToUpper(key) {
			keyCamelCase = strcase.ToLowerCamel(strings.ToLower(key))
		}
		secValues[key] = value
	}

	values = helmify.Values{
		nameCamelCase: map[string]interface{}{
			"secret": secValues,
		},
	}

	secretType := ""
	if sec.Type != "" {
		secretType = fmt.Sprintf("type: %s", string(sec.Type))
	}

	meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix("secret"))
	if err != nil {
		return true, nil, err
	}

	return true, &result{
		name: valueName,
		data: struct {
			Name string
			Type string
			Meta string
		}{
			Name: nameCamelCase,
			Type: secretType,
			Meta: meta,
		},
		values: values,
	}, nil
}

type result struct {
	name string
	data struct {
		Name string
		Type string
		Meta string
	}
	values helmify.Values
}

func (r *result) Filename() string {
	if r.name == "chart" || r.name == "" {
		return "secret.yaml"
	}
	return fmt.Sprintf("secret-%s.yaml", r.name)
}

func (r *result) Values() helmify.Values {
	return r.values
}

func (r *result) Write(writer io.Writer) error {
	return secretTempl.Execute(writer, r.data)
}
