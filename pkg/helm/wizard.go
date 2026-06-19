package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/arttor/helmify"
	"gopkg.in/yaml.v3"
)

// WriteTarGz writes the map of relative file path -> file content into a tar.gz stream.
func WriteTarGz(files map[string][]byte, chartName string, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		var path string
		if name == ".gitlab-ci.yml" {
			path = name
		} else {
			path = filepath.Join(chartName, name)
		}
		header := &tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}
	return nil
}

// GetModelDefaults returns the parsed values.yaml structure for a given chart type.
func GetModelDefaults(chartType string) (map[string]interface{}, error) {
	if chartType != "single" && chartType != "multi" {
		return nil, fmt.Errorf("invalid chart type: %s", chartType)
	}

	basePath := "models/single"
	if chartType == "multi" {
		basePath = "models/multi"
	}

	data, err := helmify.ModelsFS.ReadFile(filepath.Join(basePath, "values.yaml"))
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return m, nil
}



// WizardParams defines the JSON request payload for the Chart Generator Wizard.
type WizardParams struct {
	ChartName    string                      `json:"chartName"`
	Type         string                      `json:"type"` // "single" or "multi"
	DevRepoURL   string                      `json:"devRepoUrl"`
	GlobalConfig map[string]string           `json:"globalConfig"`
	Deployments  map[string]DeploymentParams `json:"deployments"`
}

// DeploymentParams represents configuration for a component deployment.
type DeploymentParams struct {
	Replicas *int              `json:"replicas"`
	Image    ImageParams       `json:"image"`
	Service  ServiceParams     `json:"service"`
	Cm       map[string]string `json:"cm"`
	Secret   map[string]string `json:"secret"`
	Route    RouteParams       `json:"route"`
}

// ImageParams configures the container image.
type ImageParams struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

// ServiceParams configures the internal service port.
type ServiceParams struct {
	Port int `json:"port"`
}

// RouteParams configures routing options and paths.
type RouteParams struct {
	Path     string            `json:"path"`
	Default  SubRouteParams    `json:"default"`
	Internal SubRouteParams    `json:"internal"`
	External SubRouteParams    `json:"external"`
}

// SubRouteParams configures route state and hostname.
type SubRouteParams struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
}

// GenerateWizardChart reads single or multi chart templates from the embedded ModelsFS,
// applies customization overrides to values.yaml preserving comments, renames files and
// component references, and returns a map of relative file path -> file content.
func GenerateWizardChart(params WizardParams) (map[string][]byte, error) {
	if params.ChartName == "" {
		return nil, fmt.Errorf("chartName is required")
	}
	if params.Type != "single" && params.Type != "multi" {
		return nil, fmt.Errorf("type must be 'single' or 'multi'")
	}

	basePath := "models/single"
	if params.Type == "multi" {
		basePath = "models/multi"
	}

	// 1. Walk the embedded directory and read all files
	embeddedFiles := make(map[string][]byte)
	err := fs.WalkDir(helmify.ModelsFS, basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := helmify.ModelsFS.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		embeddedFiles[relPath] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded templates: %w", err)
	}

	oldChartName := ""
	if chartYaml, ok := embeddedFiles["Chart.yaml"]; ok {
		oldChartName = getChartNameFromMetadata(chartYaml)
	}
	if oldChartName == "" {
		oldChartName = "chart-model-single"
		if params.Type == "multi" {
			oldChartName = "chart-model-multi"
		}
	}

	// 2. Setup the output map and filters
	outputFiles := make(map[string][]byte)

	// Copy non-component files (Chart.yaml, helpers, global config, .helmignore)
	for relPath, data := range embeddedFiles {
		if !strings.Contains(relPath, "templates/") || relPath == "templates/_helpers.tpl" || relPath == "templates/cm-global.yaml" {
			// Replace oldChartName with new ChartName in non-component files
			content := replaceChartName(string(data), oldChartName, params.ChartName)
			outputFiles[relPath] = []byte(content)
		}
	}

	// 3. Process component templates
	if params.Type == "single" {
		// Single deployment has a single component mapped directly to the chart itself
		var depConfig DeploymentParams
		if cfg, ok := params.Deployments[params.ChartName]; ok {
			depConfig = cfg
		} else if len(params.Deployments) > 0 {
			// fallback to first key
			for _, cfg := range params.Deployments {
				depConfig = cfg
				break
			}
		}

		// Copy templates as-is but replacing name/references
		for relPath, data := range embeddedFiles {
			if strings.Contains(relPath, "templates/") && relPath != "templates/_helpers.tpl" && relPath != "templates/cm-global.yaml" {
				content := replaceChartName(string(data), oldChartName, params.ChartName)
				outputFiles[relPath] = []byte(content)
			}
		}

		// Update values.yaml
		valuesData := embeddedFiles["values.yaml"]
		var rootNode yaml.Node
		if err := yaml.Unmarshal(valuesData, &rootNode); err != nil {
			return nil, fmt.Errorf("failed to parse values.yaml: %w", err)
		}

		// Rename root key chart-model-single to params.ChartName
		renameRootKey(&rootNode, oldChartName, params.ChartName)
		_ = setYamlPath(&rootNode, []string{"fullnameOverride"}, params.ChartName)

		// Set overrides
		appKey := params.ChartName
		if depConfig.Replicas != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "replicas"}, *depConfig.Replicas)
		}
		if depConfig.Image.Repository != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "image", "repository"}, depConfig.Image.Repository)
		}
		if depConfig.Image.Tag != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "image", "tag"}, depConfig.Image.Tag)
		}
		if depConfig.Service.Port > 0 {
			_ = setYamlPath(&rootNode, []string{appKey, "service", "port"}, depConfig.Service.Port)
		}
		if depConfig.Cm != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "cm"}, depConfig.Cm)
		}
		if depConfig.Secret != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "secret"}, depConfig.Secret)
		}
		if depConfig.Route.Path != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "route", "path"}, depConfig.Route.Path)
		}
		defaultHost, internalHost, externalHost := computeRouteHosts(params.ChartName, params.ChartName, depConfig.Route.Path, false)
		_ = setYamlPath(&rootNode, []string{appKey, "route", "default", "enabled"}, depConfig.Route.Default.Enabled)
		if depConfig.Route.Default.Host != "" {
			defaultHost = depConfig.Route.Default.Host
		}
		_ = setYamlPath(&rootNode, []string{appKey, "route", "default", "host"}, defaultHost)

		_ = setYamlPath(&rootNode, []string{appKey, "route", "internal", "enabled"}, depConfig.Route.Internal.Enabled)
		if depConfig.Route.Internal.Host != "" {
			internalHost = depConfig.Route.Internal.Host
		}
		_ = setYamlPath(&rootNode, []string{appKey, "route", "internal", "host"}, internalHost)

		_ = setYamlPath(&rootNode, []string{appKey, "route", "external", "enabled"}, depConfig.Route.External.Enabled)
		if depConfig.Route.External.Host != "" {
			externalHost = depConfig.Route.External.Host
		}
		_ = setYamlPath(&rootNode, []string{appKey, "route", "external", "host"}, externalHost)

		if len(params.GlobalConfig) > 0 {
			_ = setYamlPath(&rootNode, []string{"global"}, params.GlobalConfig)
		}

		// Re-marshal preserving comments
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&rootNode); err != nil {
			return nil, fmt.Errorf("failed to encode values.yaml: %w", err)
		}
		valuesStr := buf.String()
		valuesStr = replaceChartName(valuesStr, oldChartName, params.ChartName)
		outputFiles["values.yaml"] = []byte(valuesStr)

	} else {
		// Multi deployment supports api, web, and custom components dynamically
		valuesData := embeddedFiles["values.yaml"]
		var rootNode yaml.Node
		if err := yaml.Unmarshal(valuesData, &rootNode); err != nil {
			return nil, fmt.Errorf("failed to parse values.yaml: %w", err)
		}

		// Find the mapping node
		var mapping *yaml.Node
		if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
			mapping = rootNode.Content[0]
		}

		// Delete default components if not requested
		if _, ok := params.Deployments["backend"]; !ok {
			deleteYamlPath(&rootNode, "backend")
		}
		if _, ok := params.Deployments["frontend"]; !ok {
			deleteYamlPath(&rootNode, "frontend")
		}
		_ = setYamlPath(&rootNode, []string{"fullnameOverride"}, params.ChartName)

		// Process each user component
		for compName, depConfig := range params.Deployments {
			baseComp := "backend"
			if compName == "frontend" || compName == "web" {
				baseComp = "frontend"
			}

			// 1. Copy/rename templates
			for relPath, data := range embeddedFiles {
				if !strings.Contains(relPath, "templates/") || relPath == "templates/_helpers.tpl" || relPath == "templates/cm-global.yaml" {
					continue
				}

				filename := filepath.Base(relPath)
				if strings.Contains(filename, "-"+baseComp) || strings.Contains(filename, baseComp+"-") || strings.Contains(filename, baseComp+".") {
					newFilename := strings.Replace(filename, baseComp, compName, 1)
					newRelPath := filepath.Join("templates", newFilename)

					contentStr := string(data)
					if compName != baseComp {
						contentStr = replaceComponent(contentStr, baseComp, compName)
					}
					contentStr = replaceChartName(contentStr, oldChartName, params.ChartName)
					outputFiles[newRelPath] = []byte(contentStr)
				}
			}

			// 2. Setup values.yaml entry
			// If key doesn't exist, clone the baseComp structure from original values.yaml, or fallback to api
			exists := false
			if mapping != nil && mapping.Kind == yaml.MappingNode {
				for i := 0; i < len(mapping.Content); i += 2 {
					if mapping.Content[i].Value == compName {
						exists = true
						break
					}
				}
			}

			if !exists && mapping != nil && mapping.Kind == yaml.MappingNode {
				// Find and clone baseComp node
				var baseNode *yaml.Node
				for i := 0; i < len(mapping.Content); i += 2 {
					if mapping.Content[i].Value == baseComp {
						baseNode = mapping.Content[i+1]
						break
					}
				}
				// If baseComp node wasn't found in current mapping, find it in original values.yaml
				if baseNode == nil {
					var origRoot yaml.Node
					if err := yaml.Unmarshal(valuesData, &origRoot); err == nil && origRoot.Kind == yaml.DocumentNode && len(origRoot.Content) > 0 {
						origMapping := origRoot.Content[0]
						if origMapping.Kind == yaml.MappingNode {
							for i := 0; i < len(origMapping.Content); i += 2 {
								if origMapping.Content[i].Value == "backend" {
									baseNode = origMapping.Content[i+1]
									break
								}
							}
						}
					}
				}

				if baseNode != nil {
					cloned := cloneYamlNode(baseNode)
					keyNode := &yaml.Node{
						Kind:  yaml.ScalarNode,
						Value: compName,
					}
					mapping.Content = append(mapping.Content, keyNode, cloned)
				}
			}

			// Apply overrides to compName in values.yaml
			if depConfig.Replicas != nil {
				_ = setYamlPath(&rootNode, []string{compName, "replicas"}, *depConfig.Replicas)
			}
			if depConfig.Image.Repository != "" {
				_ = setYamlPath(&rootNode, []string{compName, "image", "repository"}, depConfig.Image.Repository)
			}
			if depConfig.Image.Tag != "" {
				_ = setYamlPath(&rootNode, []string{compName, "image", "tag"}, depConfig.Image.Tag)
			}
			if depConfig.Service.Port > 0 {
				_ = setYamlPath(&rootNode, []string{compName, "service", "port"}, depConfig.Service.Port)
			}
			if depConfig.Cm != nil {
				_ = setYamlPath(&rootNode, []string{compName, "cm"}, depConfig.Cm)
			}
			if depConfig.Secret != nil {
				_ = setYamlPath(&rootNode, []string{compName, "secret"}, depConfig.Secret)
			}
			if depConfig.Route.Path != "" {
				_ = setYamlPath(&rootNode, []string{compName, "route", "path"}, depConfig.Route.Path)
			}
			defaultHost, internalHost, externalHost := computeRouteHosts(params.ChartName, compName, depConfig.Route.Path, true)
			_ = setYamlPath(&rootNode, []string{compName, "route", "default", "enabled"}, depConfig.Route.Default.Enabled)
			if depConfig.Route.Default.Host != "" {
				defaultHost = depConfig.Route.Default.Host
			}
			_ = setYamlPath(&rootNode, []string{compName, "route", "default", "host"}, defaultHost)

			_ = setYamlPath(&rootNode, []string{compName, "route", "internal", "enabled"}, depConfig.Route.Internal.Enabled)
			if depConfig.Route.Internal.Host != "" {
				internalHost = depConfig.Route.Internal.Host
			}
			_ = setYamlPath(&rootNode, []string{compName, "route", "internal", "host"}, internalHost)

			_ = setYamlPath(&rootNode, []string{compName, "route", "external", "enabled"}, depConfig.Route.External.Enabled)
			if depConfig.Route.External.Host != "" {
				externalHost = depConfig.Route.External.Host
			}
			_ = setYamlPath(&rootNode, []string{compName, "route", "external", "host"}, externalHost)
		}

		if len(params.GlobalConfig) > 0 {
			_ = setYamlPath(&rootNode, []string{"global"}, params.GlobalConfig)
		}

		// Re-marshal values.yaml preserving comments
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&rootNode); err != nil {
			return nil, fmt.Errorf("failed to encode values.yaml: %w", err)
		}
		valuesStr := buf.String()
		// Replace chart name inside values.yaml (e.g. in affinity matching labels)
		valuesStr = replaceChartName(valuesStr, oldChartName, params.ChartName)
		outputFiles["values.yaml"] = []byte(valuesStr)
	}

	if chartData, ok := outputFiles["Chart.yaml"]; ok && params.DevRepoURL != "" {
		var chartNode yaml.Node
		if err := yaml.Unmarshal(chartData, &chartNode); err == nil {
			_ = setYamlPath(&chartNode, []string{"annotations", "tjpa.jus.br/dev-source-repo"}, params.DevRepoURL)
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&chartNode); err == nil {
				outputFiles["Chart.yaml"] = buf.Bytes()
			}
		}
	}

	if valuesData, ok := outputFiles["values.yaml"]; ok {
		var valuesNode yaml.Node
		if err := yaml.Unmarshal(valuesData, &valuesNode); err == nil {
			if devVals, err := generateDevValues(&valuesNode); err == nil {
				outputFiles["values-ca.yaml"] = devVals
			}
		}
	}

	outputFiles[".gitlab-ci.yml"] = helmify.GitLabCI

	return outputFiles, nil
}

func replaceChartName(content string, oldChartName, newChartName string) string {
	res := strings.ReplaceAll(content, oldChartName, newChartName)
	res = strings.ReplaceAll(res, "chart-model-single", newChartName)
	res = strings.ReplaceAll(res, "chart-model-multi", newChartName)
	res = strings.ReplaceAll(res, "chart-model", newChartName)
	return res
}

func replaceComponent(content string, oldComp, newComp string) string {
	repls := []struct{ old, new string }{
		{"chart-model-multi.fullname\" . }}-" + oldComp, "chart-model-multi.fullname\" . }}-" + newComp},
		{"-" + oldComp + "-cm", "-" + newComp + "-cm"},
		{"-" + oldComp + "-secret", "-" + newComp + "-secret"},
		{"component: " + oldComp, "component: " + newComp},
		{"name: " + oldComp, "name: " + newComp},
		{"cm-" + oldComp + ".yaml", "cm-" + newComp + ".yaml"},
		{"secret-" + oldComp + ".yaml", "secret-" + newComp + ".yaml"},
		{".Values." + oldComp, ".Values." + newComp},
	}
	res := content
	for _, r := range repls {
		res = strings.ReplaceAll(res, r.old, r.new)
	}
	return res
}

func cloneYamlNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	cloned := *node
	if len(node.Content) > 0 {
		cloned.Content = make([]*yaml.Node, len(node.Content))
		for i, c := range node.Content {
			cloned.Content[i] = cloneYamlNode(c)
		}
	}
	return &cloned
}

func renameRootKey(node *yaml.Node, oldKey, newKey string) {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		renameRootKey(node.Content[0], oldKey, newKey)
		return
	}
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == oldKey {
			node.Content[i].Value = newKey
			return
		}
	}
}

func deleteYamlPath(node *yaml.Node, key string) {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		deleteYamlPath(node.Content[0], key)
		return
	}
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

func setYamlPath(node *yaml.Node, path []string, val interface{}) error {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return fmt.Errorf("empty document node")
		}
		return setYamlPath(node.Content[0], path, val)
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got kind %v", node.Kind)
	}
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	key := path[0]
	for i := 0; i < len(node.Content); i += 2 {
		kNode := node.Content[i]
		if kNode.Value == key {
			if len(path) == 1 {
				var valNode yaml.Node
				b, err := yaml.Marshal(val)
				if err != nil {
					return err
				}
				if err := yaml.Unmarshal(b, &valNode); err != nil {
					return err
				}
				var insertValNode *yaml.Node
				if len(valNode.Content) > 0 {
					insertValNode = valNode.Content[0]
				} else {
					insertValNode = &valNode
				}
				node.Content[i+1] = insertValNode
				return nil
			}
			return setYamlPath(node.Content[i+1], path[1:], val)
		}
	}

	if len(path) == 1 {
		var valNode yaml.Node
		b, err := yaml.Marshal(val)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(b, &valNode); err != nil {
			return err
		}
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}
		var insertValNode *yaml.Node
		if len(valNode.Content) > 0 {
			insertValNode = valNode.Content[0]
		} else {
			insertValNode = &valNode
		}
		node.Content = append(node.Content, keyNode, insertValNode)
		return nil
	}

	newMap := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
	node.Content = append(node.Content, keyNode, newMap)
	return setYamlPath(newMap, path[1:], val)
}

func getChartNameFromMetadata(chartYaml []byte) string {
	var meta struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(chartYaml, &meta); err == nil && meta.Name != "" {
		return meta.Name
	}
	return ""
}
