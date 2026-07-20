package helm

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// generateDevValues extracts the global configuration, plus 'cm' and 'secret' blocks
// for all component deployments, and packages them into a separate dev-friendly values-dev.yaml file.
func generateDevValues(rootNode *yaml.Node) ([]byte, error) {
	var origMap *yaml.Node
	if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
		origMap = rootNode.Content[0]
	} else if rootNode.Kind == yaml.MappingNode {
		origMap = rootNode
	}
	if origMap == nil || origMap.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("invalid root node")
	}

	var devRoot yaml.Node
	devRoot.Kind = yaml.DocumentNode
	devMap := yaml.Node{
		Kind: yaml.MappingNode,
	}
	devRoot.Content = append(devRoot.Content, &devMap)

	// Add header comments
	devMap.HeadComment = "Standardized TJPA Application Values (values-ca.yaml)\n" +
		"This file contains application-specific environment variables (cm) and credentials (secret).\n" +
		"Developers can manage their ConfigMaps and Secrets here, separating dev/app configs from the core infrastructure."

	for i := 0; i < len(origMap.Content); i += 2 {
		keyNode := origMap.Content[i]
		valNode := origMap.Content[i+1]

		if keyNode.Value == "global" {
			devMap.Content = append(devMap.Content, cloneYamlNode(keyNode), cloneYamlNode(valNode))
			continue
		}

		// Check if it's a component map (contains other sub-keys)
		if valNode.Kind == yaml.MappingNode {
			compMap := yaml.Node{
				Kind: yaml.MappingNode,
			}
			hasContent := false
			for j := 0; j < len(valNode.Content); j += 2 {
				subKey := valNode.Content[j]
				subVal := valNode.Content[j+1]
				if subKey.Value == "cm" || subKey.Value == "secret" {
					compMap.Content = append(compMap.Content, cloneYamlNode(subKey), cloneYamlNode(subVal))
					hasContent = true
				}
			}
			if hasContent {
				devMap.Content = append(devMap.Content, cloneYamlNode(keyNode), &compMap)
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&devRoot); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
