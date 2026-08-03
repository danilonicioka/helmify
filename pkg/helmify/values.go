package helmify

import (
	"fmt"
	"strconv"
	"strings"

	"dario.cat/mergo"

	

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Values - represents helm template values.yaml.
type Values map[string]interface{}

// Merge given values with current instance.
func (v *Values) Merge(values Values) error {
	if err := mergo.Merge(v, values, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("%w: unable to merge helm values", err)
	}
	return nil
}

// Add - adds given value to values and returns its helm template representation {{ .Values.<valueName> }}
func (v *Values) Add(value interface{}, name ...string) (string, error) {
		switch val := value.(type) {
	case int:
		value = int64(val)
	case int8:
		value = int64(val)
	case int16:
		value = int64(val)
	case int32:
		value = int64(val)
	}

	err := unstructured.SetNestedField(*v, value, name...)
	if err != nil {
		return "", fmt.Errorf("%w: unable to set value: %v", err, name)
	}
	_, isString := value.(string)
	if isString {
		return "{{ " + formatValuesPath(name) + " | quote }}", nil
	}
	_, isSlice := value.([]interface{})
	if isSlice {
		spaces := strconv.Itoa(len(name) * 2)
		return "{{ toYaml " + formatValuesPath(name) + " | nindent " + spaces + " }}", nil
	}
	return "{{ " + formatValuesPath(name) + " }}", nil
}

// AddYaml - adds given value to values and returns its helm template representation as Yaml {{ .Values.<valueName> | toYaml | indent i }}
// indent  <= 0 will be omitted.
func (v *Values) AddYaml(value interface{}, indent int, newLine bool, name ...string) (string, error) {
		err := unstructured.SetNestedField(*v, value, name...)
	if err != nil {
		return "", fmt.Errorf("%w: unable to set value: %v", err, name)
	}
	if indent > 0 {
		if newLine {
			return "{{ " + formatValuesPath(name) + fmt.Sprintf(" | toYaml | nindent %d }}", indent), nil
		}
		return "{{ " + formatValuesPath(name) + fmt.Sprintf(" | toYaml | indent %d }}", indent), nil
	}
	return "{{ " + formatValuesPath(name) + " | toYaml }}", nil
}

// AddSecret - adds empty value to values and returns its helm template representation {{ required "<valueName>" .Values.<valueName> }}.
// Set toBase64=true for Secret data to be base64 encoded and set false for Secret stringData.
func (v *Values) AddSecret(toBase64 bool, name ...string) (string, error) {
		nameStr := strings.Join(name, ".")
	err := unstructured.SetNestedField(*v, "", name...)
	if err != nil {
		return "", fmt.Errorf("%w: unable to set value: %v", err, nameStr)
	}
	res := fmt.Sprintf(`{{ required "%[1]s is required" %s`, nameStr, formatValuesPath(name))
	if toBase64 {
		res += " | b64enc"
	}
	return res + " | quote }}", err
}



func formatValuesPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	res := fmt.Sprintf(`(index .Values "%s")`, path[0])
	if len(path) > 1 {
		res += "." + strings.Join(path[1:], ".")
	}
	return res
}
