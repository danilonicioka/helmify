package helm

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/danilonicioka/helmify/pkg/helmify"
	"gopkg.in/yaml.v3"
)

func toMapStringInterface(v interface{}) (map[string]interface{}, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	if m, ok := v.(helmify.Values); ok {
		return map[string]interface{}(m), true
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		res := make(map[string]interface{})
		for k, val := range m {
			res[fmt.Sprintf("%v", k)] = val
		}
		return res, true
	}
	return nil, false
}

// mergeDevValues injects the 'cm' blocks from the dynamically generated values
// into the static values-ca.yaml node tree to preserve comments and structure.
func mergeDevValues(caData []byte, chartName string, values helmify.Values, valuesYamlData []byte) ([]byte, error) {
	caStr := string(caData)
	caStr = strings.ReplaceAll(caStr, "chart-model-single", chartName)
	caStr = strings.ReplaceAll(caStr, "chart-model-multi", chartName)

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(caStr), &node); err != nil {
		return nil, err
	}

	for k, v := range values {
		if valMap, ok := toMapStringInterface(v); ok {
			if cmVal, ok := valMap["cm"]; ok {
				if cmMap, ok := toMapStringInterface(cmVal); ok {
					if len(cmMap) > 0 {
						if err := mergeYamlNode(&node, cmMap, []string{k, "cm"}); err != nil {
							return nil, err
						}
					}
				}
			}
			if filesVal, ok := valMap["files"]; ok {
				if filesMap, ok := toMapStringInterface(filesVal); ok {
					if filesCmVal, ok := filesMap["cm"]; ok {
						if filesCmMap, ok := toMapStringInterface(filesCmVal); ok {
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

	// Sort the top-level keys of values-ca.yaml to match the exact order of values.yaml
	var valuesYamlNode yaml.Node
	if err := yaml.Unmarshal(valuesYamlData, &valuesYamlNode); err == nil && valuesYamlNode.Kind == yaml.DocumentNode && len(valuesYamlNode.Content) > 0 {
		valuesRoot := valuesYamlNode.Content[0]
		if valuesRoot.Kind == yaml.MappingNode {
			orderMap := make(map[string]int)
			for i := 0; i < len(valuesRoot.Content); i += 2 {
				k := valuesRoot.Content[i]
				orderMap[k.Value] = i
			}

			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				root := node.Content[0]
				if root.Kind == yaml.MappingNode {
					type kv struct {
						key *yaml.Node
						val *yaml.Node
						idx int
					}
					var kvs []kv
					for i := 0; i < len(root.Content); i += 2 {
						k := root.Content[i]
						v := root.Content[i+1]
						idx, ok := orderMap[k.Value]
						if !ok {
							continue // Drop keys that do not exist in values.yaml
						}
						kvs = append(kvs, kv{key: k, val: v, idx: idx})
					}
					sort.SliceStable(kvs, func(i, j int) bool {
						return kvs[i].idx < kvs[j].idx
					})

					var newContent []*yaml.Node
					for _, kv := range kvs {
						newContent = append(newContent, kv.key, kv.val)
					}
					root.Content = newContent
				}
			}
		}
	}

	setBlockStyle(&node)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
