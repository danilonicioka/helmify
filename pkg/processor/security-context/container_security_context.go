package security_context

import (
	"fmt"
	"strings"

	"github.com/arttor/helmify/pkg/helmify"
	"github.com/iancoleman/strcase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	sc           = "securityContext"
	cscValueName = "containerSecurityContext"
	helmTemplate = "{{- with .Values.%[1]s.containerSecurityContext }}\nsecurityContext:\n  {{- toYaml . | nindent %[3]d }}\n{{- end }}"
)

// ProcessContainerSecurityContext adds 'securityContext' to the podSpec in specMap, if it doesn't have one already defined.
func ProcessContainerSecurityContext(nameCamel string, specMap map[string]interface{}, values *helmify.Values, nindent int) error {
	err := processSecurityContext(nameCamel, "containers", specMap, values, nindent)
	if err != nil {
		return err
	}

	err = processSecurityContext(nameCamel, "initContainers", specMap, values, nindent)
	if err != nil {
		return err
	}

	return nil
}

func processSecurityContext(nameCamel string, containerType string, specMap map[string]interface{}, values *helmify.Values, nindent int) error {
	if containers, defined := specMap[containerType]; defined {
		for _, container := range containers.([]interface{}) {
			castedContainer := container.(map[string]interface{})
			containerName := strcase.ToLowerCamel(castedContainer["name"].(string))
			if _, defined2 := castedContainer["securityContext"]; defined2 {
				err := setSecContextValue(nameCamel, containerName, castedContainer, values, nindent)
				if err != nil {
					return err
				}
			}
		}
		err := unstructured.SetNestedField(specMap, containers, containerType)
		if err != nil {
			return err
		}
	}
	return nil
}

func setSecContextValue(resourceName string, containerName string, castedContainer map[string]interface{}, values *helmify.Values, nindent int) error {
	var valuePath []string
	if containerName == resourceName || containerName == "" {
		valuePath = []string{resourceName}
	} else {
		valuePath = []string{resourceName, containerName}
	}
	valuePathStr := strings.Join(valuePath, ".")

	// Initialize as empty object {} per Zero-Default standard
	err := unstructured.SetNestedField(*values, map[string]interface{}{}, append(valuePath, cscValueName)...)
	if err != nil {
		return err
	}

	valueString := fmt.Sprintf(helmTemplate, valuePathStr, 0, nindent+2)
	castedContainer[sc] = valueString
	return nil
}
