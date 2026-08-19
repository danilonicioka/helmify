package helm

import (
	"bytes"
	"strings"

	"github.com/danilonicioka/helmify/pkg/helmify"
	"gopkg.in/yaml.v3"
)

// mergeDevValues injects the 'cm' blocks from the dynamically generated values
// into the static values-ca.yaml node tree to preserve comments and structure.
func mergeDevValues(caData []byte, chartName string, values helmify.Values) ([]byte, error) {
	caStr := string(caData)
	caStr = strings.ReplaceAll(caStr, "chart-model-single", chartName)
	caStr = strings.ReplaceAll(caStr, "chart-model-multi", chartName)

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(caStr), &node); err != nil {
		return nil, err
	}

	for k, v := range values {
		if valMap, ok := v.(map[string]interface{}); ok {
			if cmVal, ok := valMap["cm"]; ok {
				if cmMap, ok := cmVal.(map[string]interface{}); ok {
					if len(cmMap) > 0 {
						if err := mergeYamlNode(&node, cmMap, []string{k, "cm"}); err != nil {
							return nil, err
						}
					}
				}
			}
			if filesVal, ok := valMap["files"]; ok {
				if filesMap, ok := filesVal.(map[string]interface{}); ok {
					if filesCmVal, ok := filesMap["cm"]; ok {
						if filesCmMap, ok := filesCmVal.(map[string]interface{}); ok {
							if len(filesCmMap) > 0 {
								if err := mergeYamlNode(&node, filesCmMap, []string{k, "files", "cm"}); err != nil {
									return nil, err
								}
							}
						}
					}
				}
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
