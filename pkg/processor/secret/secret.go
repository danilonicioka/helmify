package secret

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/arttor/helmify/pkg/processor"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var secretTempl = template.Must(template.New("secret").Funcs(sprig.TxtFuncMap()).Parse(
	`{{- if .IsGlobal -}}
{{- if and .Values.global .Values.global.secret -}}
{{ .Meta }}
{{- if .Type }}
{{ .Type }}
{{- end }}
data:
{{- range $key, $value := .Values.global.secret }}
  {{ $key }}: {{ $value | b64enc | quote }}
{{- end }}
{{- end }}
{{- else -}}
{{ "{" }}{{ "{" }}- if and .Values.{{ .Name }} .Values.{{ .Name }}.secret {{ "}" }}{{ "}" }}
{{ .Meta }}
{{- if .Type }}
{{ .Type }}
{{- end }}
data:
{{ "{" }}{{ "{" }}- range $key, $value := .Values.{{ .Name }}.secret {{ "}" }}{{ "}" }}
  {{ "{{ $key }}" }}: {{ "{{ $value | b64enc | quote }}" }}
{{ "{" }}{{ "{" }}- end {{ "}" }}{{ "}" }}
{{ "{" }}{{ "{" }}- end {{ "}" }}{{ "}" }}
{{- end }}`))

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

	referencingComps := processor.FindReferencingComponents(appMeta, obj.GetName(), true)

	nameLower := strings.ToLower(obj.GetName())
	isGlobal := false
	if strings.HasSuffix(nameLower, "global") || strings.HasSuffix(nameLower, "global-secrets") || strings.HasSuffix(nameLower, "secrets-global") || strings.HasSuffix(nameLower, "secret-global") || len(referencingComps) > 1 {
		isGlobal = true
	}

	secretType := ""
	if sec.Type != "" {
		secretType = fmt.Sprintf("type: %s", string(sec.Type))
	}

	if isGlobal {
		globalValues := map[string]interface{}{}
		for key, value := range sec.Data {
			globalValues[key] = string(value)
		}
		for key, value := range sec.StringData {
			globalValues[key] = value
		}
		values := helmify.Values{
			"global": map[string]interface{}{
				"secret": globalValues,
			},
		}

		suffix := "global"
		meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
		if err != nil {
			return true, nil, err
		}

		return true, &secretTemplate{
			compName:   "global",
			secretType: secretType,
			meta:       meta,
			values:     values,
			isGlobal:   true,
		}, nil
	}

	if len(referencingComps) == 0 {
		compName := processor.GetComponent(obj)
		if compName != "" && compName != "chart" && compName != "secrets" {
			referencingComps = []string{compName}
		}
	}

	if len(referencingComps) == 0 {
		// If no components reference this secret, we skip writing it to prevent pollution
		return true, nil, nil
	}

	secValues := map[string]interface{}{}
	for key, value := range sec.Data {
		secValues[key] = string(value)
	}
	for key, value := range sec.StringData {
		secValues[key] = value
	}

	var templates []helmify.Template
	for _, comp := range referencingComps {
		nameCamelCase := strcase.ToLowerCamel(comp)
		values := helmify.Values{
			nameCamelCase: map[string]interface{}{
				"secret": secValues,
			},
		}

		suffix := comp + "-secrets"
		meta, err := processor.ProcessObjMeta(appMeta, obj, processor.WithSuffix(suffix))
		if err != nil {
			return true, nil, err
		}

		templates = append(templates, &secretTemplate{
			compName:      comp,
			nameCamelCase: nameCamelCase,
			secretType:    secretType,
			meta:          meta,
			values:        values,
			isGlobal:      false,
		})
	}

	return true, &multiTemplate{templates: templates}, nil
}

type secretTemplate struct {
	compName      string
	nameCamelCase string
	secretType    string
	meta          string
	values        helmify.Values
	isGlobal      bool
}

func (s *secretTemplate) Filename() string {
	if s.isGlobal {
		return "secret-global.yaml"
	}
	return fmt.Sprintf("secret-%s.yaml", s.compName)
}

func (s *secretTemplate) Values() helmify.Values {
	return s.values
}

func (s *secretTemplate) Write(writer io.Writer) error {
	return secretTempl.Execute(writer, struct {
		Name     string
		Type     string
		Meta     string
		Values   helmify.Values
		IsGlobal bool
	}{
		Name:     s.nameCamelCase,
		Type:     s.secretType,
		Meta:     s.meta,
		Values:   s.values,
		IsGlobal: s.isGlobal,
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
