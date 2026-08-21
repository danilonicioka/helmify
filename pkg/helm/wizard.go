package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	roothelmify "github.com/danilonicioka/helmify"
	"github.com/danilonicioka/helmify/pkg/config"
	"github.com/danilonicioka/helmify/pkg/helmify"
	"github.com/danilonicioka/helmify/pkg/processor"
	"gopkg.in/yaml.v3"
)

// WriteTarGz writes the map of relative file path -> file content into a tar.gz stream.
func WriteTarGz(files map[string][]byte, chartName string, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Collect and sort file names for deterministic output
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)

	// Track directories that have been written to avoid duplicates
	writtenDirs := make(map[string]bool)

	var ensureDir func(string) error
	ensureDir = func(dir string) error {
		if dir == "" || writtenDirs[dir] {
			return nil
		}
		// Recursively ensure parent directories
		parent := filepath.Dir(dir)
		if parent != "." && parent != dir {
			if err := ensureDir(parent); err != nil {
				return err
			}
		}
		header := &tar.Header{Name: dir + "/", Mode: 0755, Typeflag: tar.TypeDir}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing dir %s: %w", dir, err)
		}
		writtenDirs[dir] = true
		return nil
	}

	for _, name := range names {
		if name == "templates" {
			continue
		}
		content := files[name]
		contentStr := string(content)
		contentStr = strings.ReplaceAll(contentStr, "{{REGISTRY}}", config.GlobalEnvConfig.Registry)
		contentStr = strings.ReplaceAll(contentStr, "{{DEFAULT_DOMAIN}}", config.GlobalEnvConfig.DefaultDomain)
		contentStr = strings.ReplaceAll(contentStr, "{{INTERNAL_DOMAIN}}", config.GlobalEnvConfig.InternalDomain)
		contentStr = strings.ReplaceAll(contentStr, "{{EXTERNAL_DOMAIN}}", config.GlobalEnvConfig.ExternalDomain)
		content = []byte(contentStr)

		var pathStr string
		if name == ".gitlab-ci.yml" || name == "README.md" {
			pathStr = name
		} else {
			pathStr = filepath.Join("chart", name)
		}

		// Ensure directory for this file exists
		if dir := filepath.Dir(pathStr); dir != "." && dir != "" {
			if err := ensureDir(dir); err != nil {
				return err
			}
		}

		header := &tar.Header{
			Name: pathStr,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing %s: %w", pathStr, err)
		}
		if _, err := tw.Write(content); err != nil {
			return fmt.Errorf("writing %s body: %w", pathStr, err)
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

	data, err := roothelmify.ModelsFS.ReadFile(filepath.Join(basePath, "values.yaml"))
	if err != nil {
		return nil, err
	}

	dataStr := string(data)
	dataStr = strings.ReplaceAll(dataStr, "{{REGISTRY}}", config.GlobalEnvConfig.Registry)
	dataStr = strings.ReplaceAll(dataStr, "{{DEFAULT_DOMAIN}}", config.GlobalEnvConfig.DefaultDomain)
	dataStr = strings.ReplaceAll(dataStr, "{{INTERNAL_DOMAIN}}", config.GlobalEnvConfig.InternalDomain)
	dataStr = strings.ReplaceAll(dataStr, "{{EXTERNAL_DOMAIN}}", config.GlobalEnvConfig.ExternalDomain)

	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(dataStr), &m); err != nil {
		return nil, err
	}

	return m, nil
}

// WizardParams defines the JSON request payload for the Chart Generator Wizard.
type WizardParams struct {
	ChartName     string                      `json:"chartName"`
	Type          string                      `json:"type"` // "single" or "multi"
	DevRepoURL    string                      `json:"devRepoUrl"`
	GlobalConfig  map[string]string           `json:"globalConfig"`
	GlobalSecret  map[string]string           `json:"globalSecret"`
	Deployments   map[string]DeploymentParams `json:"deployments"`
	Subcomponents []string                    `json:"subcomponents"`
}

// DeploymentParams represents configuration for a component deployment.
type DeploymentParams struct {
	Replicas         *int                        `json:"replicas"`
	Image            ImageParams                 `json:"image"`
	Service          ServiceParams               `json:"service"`
	Cm               map[string]string           `json:"cm"`
	Secret           map[string]string           `json:"secret"`
	Resources        *ResourceParams             `json:"resources,omitempty"`
	Persistence      PersistenceParams           `json:"persistence"`
	Hpa              *HpaParams                  `json:"hpa,omitempty"`
	StartupProbe     *ProbeParams                `json:"startupProbe,omitempty"`
	LivenessProbe    *ProbeParams                `json:"livenessProbe,omitempty"`
	ReadinessProbe   *ProbeParams                `json:"readinessProbe,omitempty"`
	Affinity         *AffinityParams             `json:"affinity,omitempty"`
	NodeSelector     map[string]interface{}      `json:"nodeSelector,omitempty"`
	Tolerations      []interface{}               `json:"tolerations,omitempty"`
	Route            RouteParams                 `json:"route"`
	ConnectsTo       []string                    `json:"connectsTo"`
	Runtime          string                      `json:"runtime"`
	RuntimeNamespace string                      `json:"runtimeNamespace"`
	RuntimeVersion   string                      `json:"runtimeVersion"`
	OverviewAppRoute string                      `json:"overviewAppRoute"`
	Files            CustomFiles                 `json:"files"`
}

// CustomFiles holds cm and secret files
type CustomFiles struct {
	Cm     map[string]CustomFileParams `json:"cm"`
	Secret map[string]CustomFileParams `json:"secret"`
}

// CustomFileParams defines custom file injection.
type CustomFileParams struct {
	MountPath string `json:"mountPath"`
	Content   string `json:"content"`
}

// ResourceParams configures container resource requests and limits.
type ResourceParams struct {
	Limits   map[string]interface{} `json:"limits,omitempty"`
	Requests map[string]interface{} `json:"requests,omitempty"`
}

// ImageParams configures the container image.
type ImageParams struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

// ServiceParams configures the internal service port.
type ServiceParams struct {
	Port       int `json:"port"`
	TargetPort int `json:"targetPort,omitempty"`
	Ports      map[string]struct {
		Port       int `json:"port"`
		TargetPort int `json:"targetPort,omitempty"`
	} `json:"ports"`
}

// ProbeParams enforces a strict ordering of probe fields in the generated YAML
type ProbeParams struct {
	HTTPGet             interface{} `json:"httpGet,omitempty" yaml:"httpGet,omitempty"`
	TCPSocket           interface{} `json:"tcpSocket,omitempty" yaml:"tcpSocket,omitempty"`
	Exec                interface{} `json:"exec,omitempty" yaml:"exec,omitempty"`
	InitialDelaySeconds interface{} `json:"initialDelaySeconds,omitempty" yaml:"initialDelaySeconds,omitempty"`
	PeriodSeconds       interface{} `json:"periodSeconds,omitempty" yaml:"periodSeconds,omitempty"`
	TimeoutSeconds      interface{} `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
	SuccessThreshold    interface{} `json:"successThreshold,omitempty" yaml:"successThreshold,omitempty"`
	FailureThreshold    interface{} `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`
}

// HpaParams enforces a strict ordering of HPA fields
type HpaParams struct {
	Enabled     bool        `json:"enabled" yaml:"enabled"`
	MinReplicas int         `json:"minReplicas,omitempty" yaml:"minReplicas,omitempty"`
	MaxReplicas int         `json:"maxReplicas,omitempty" yaml:"maxReplicas,omitempty"`
	Metrics     interface{} `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Behavior    interface{} `json:"behavior,omitempty" yaml:"behavior,omitempty"`
}

// AffinityParams enforces strict ordering for Affinity fields
type AffinityParams struct {
	NodeAffinity    interface{} `json:"nodeAffinity,omitempty" yaml:"nodeAffinity,omitempty"`
	PodAffinity     interface{} `json:"podAffinity,omitempty" yaml:"podAffinity,omitempty"`
	PodAntiAffinity interface{} `json:"podAntiAffinity,omitempty" yaml:"podAntiAffinity,omitempty"`
}

// RouteParams configures routing options and paths.
type RouteParams struct {
	Path     string         `json:"path"`
	Default  SubRouteParams `json:"default"`
	Internal SubRouteParams `json:"internal"`
	External SubRouteParams `json:"external"`
}

// SubRouteParams configures route state and hostname.
type SubRouteParams struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
}

// PersistenceParams configures PVC persistence.
type PersistenceParams struct {
	Enabled        bool   `json:"enabled"`
	Ephemeral      bool   `json:"ephemeral"`
	MountPath      string `json:"mountPath"`
	StorageRequest string `json:"storageRequest"`
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
	err := fs.WalkDir(roothelmify.ModelsFS, basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := roothelmify.ModelsFS.ReadFile(path)
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
		if !strings.Contains(relPath, "templates/") || relPath == "templates/_helpers.tpl" || relPath == "templates/cm-global.yaml" || relPath == "templates/secret-global.yaml" {
			content := replaceChartName(string(data), oldChartName, params.ChartName)
			outputFiles[relPath] = []byte(content)
		}
	}

	// Inject subcomponent templates
	for _, sub := range params.Subcomponents {
		subPath := fmt.Sprintf("models/subcomponents/%s/templates", sub)
		_ = fs.WalkDir(roothelmify.ModelsFS, subPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			data, err := roothelmify.ModelsFS.ReadFile(path)
			if err == nil {
				outRelPath := filepath.Join("templates", filepath.Base(path))
				content := strings.ReplaceAll(string(data), "<CHART_NAME>", params.ChartName)
				outputFiles[outRelPath] = []byte(content)
			}
			return nil
		})
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
			if strings.Contains(relPath, "templates/") && relPath != "templates/_helpers.tpl" && relPath != "templates/cm-global.yaml" && relPath != "templates/secret-global.yaml" {
				base := filepath.Base(relPath)
				if strings.HasSuffix(base, "-redis.yaml") || strings.HasSuffix(base, "-postgres.yaml") {
					continue
				}
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
		svcPort := depConfig.Service.Port
		svcTargetPort := depConfig.Service.TargetPort
		if svcPort == 0 && depConfig.Service.Ports != nil {
			if httpPort, ok := depConfig.Service.Ports["http"]; ok {
				svcPort = httpPort.Port
				svcTargetPort = httpPort.TargetPort
			}
		}
		if svcPort > 0 {
			if svcTargetPort == 0 {
				svcTargetPort = 8080 // Standard convention fallback
			}
			_ = setYamlPath(&rootNode, []string{appKey, "service", "ports", "http", "port"}, svcPort)
			_ = setYamlPath(&rootNode, []string{appKey, "service", "ports", "http", "targetPort"}, svcTargetPort)
			_ = setYamlPath(&rootNode, []string{appKey, "startupProbe", "tcpSocket", "port"}, svcTargetPort)
			_ = setYamlPath(&rootNode, []string{appKey, "livenessProbe", "tcpSocket", "port"}, svcTargetPort)
			_ = setYamlPath(&rootNode, []string{appKey, "readinessProbe", "tcpSocket", "port"}, svcTargetPort)
		}
		if depConfig.Cm != nil {
			stripQuotesFromMap(depConfig.Cm)
			_ = setYamlPath(&rootNode, []string{appKey, "cm"}, depConfig.Cm)
		}
		if depConfig.Secret != nil {
			stripQuotesFromMap(depConfig.Secret)
			_ = setYamlPath(&rootNode, []string{appKey, "secret"}, depConfig.Secret)
		}
		if depConfig.Resources != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "resources"}, depConfig.Resources)
		}
		if depConfig.Hpa != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "hpa"}, depConfig.Hpa)
		}
		if depConfig.StartupProbe != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "startupProbe"}, depConfig.StartupProbe)
		}
		if depConfig.LivenessProbe != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "livenessProbe"}, depConfig.LivenessProbe)
		}
		if depConfig.ReadinessProbe != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "readinessProbe"}, depConfig.ReadinessProbe)
		}
		if depConfig.Affinity != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "affinity"}, depConfig.Affinity)
		}
		if depConfig.NodeSelector != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "nodeSelector"}, depConfig.NodeSelector)
		}
		if depConfig.Tolerations != nil {
			_ = setYamlPath(&rootNode, []string{appKey, "tolerations"}, depConfig.Tolerations)
		}
		if depConfig.Route.Path != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "route", "path"}, depConfig.Route.Path)
		}
		for _, sub := range params.Subcomponents {
			depConfig.ConnectsTo = append(depConfig.ConnectsTo, params.ChartName+"-"+sub)
		}
		if len(depConfig.ConnectsTo) > 0 {
			var connects []string
			for _, c := range depConfig.ConnectsTo {
				connects = append(connects, fmt.Sprintf(`{"apiVersion":"apps/v1","kind":"Deployment","name":"%s"}`, c))
			}
			_ = setYamlPath(&rootNode, []string{appKey, "annotations", "app.openshift.io/connects-to"}, "["+strings.Join(connects, ",")+"]")
		}
		// Set runtime properties directly under the standard labels map
		if depConfig.Runtime != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "labels", "app.openshift.io/runtime"}, depConfig.Runtime)
		}
		if depConfig.RuntimeNamespace != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "labels", "app.openshift.io/runtime-namespace"}, depConfig.RuntimeNamespace)
		}
		if depConfig.RuntimeVersion != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "labels", "app.openshift.io/runtime-version"}, depConfig.RuntimeVersion)
		}
		// Add primary route annotation if supplied
		if depConfig.OverviewAppRoute != "" {
			_ = setYamlPath(&rootNode, []string{appKey, "annotations", "console.alpha.openshift.io/overview-app-route"}, depConfig.OverviewAppRoute)
		}

		if depConfig.Persistence.Enabled {
			_ = setYamlPath(&rootNode, []string{appKey, "persistence", "enabled"}, true)
			if depConfig.Persistence.Ephemeral {
				_ = setYamlPath(&rootNode, []string{appKey, "persistence", "ephemeral"}, true)
			}
			if depConfig.Persistence.MountPath != "" {
				_ = setYamlPath(&rootNode, []string{appKey, "persistence", "mountPath"}, depConfig.Persistence.MountPath)
			}
			if !depConfig.Persistence.Ephemeral {
				if depConfig.Persistence.StorageRequest != "" {
					_ = setYamlPath(&rootNode, []string{appKey, "persistence", "storageRequest"}, depConfig.Persistence.StorageRequest)
				}
				_ = setYamlPath(&rootNode, []string{appKey, "strategy"}, map[string]string{"type": "Recreate"})
			}
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

		if len(depConfig.Files.Cm) > 0 {
			_ = setYamlPath(&rootNode, []string{appKey, "files", "cm"}, depConfig.Files.Cm)
		}
		if len(depConfig.Files.Secret) > 0 {
			_ = setYamlPath(&rootNode, []string{appKey, "files", "secret"}, depConfig.Files.Secret)
		}

		if len(params.GlobalConfig) > 0 {
			stripQuotesFromMap(params.GlobalConfig)
			_ = setYamlPath(&rootNode, []string{"global", "cm"}, params.GlobalConfig)
		}
		if len(params.GlobalSecret) > 0 {
			stripQuotesFromMap(params.GlobalSecret)
			_ = setYamlPath(&rootNode, []string{"global", "secret"}, params.GlobalSecret)
		}

		// Re-marshal preserving comments
		setBlockStyle(&rootNode)
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&rootNode); err != nil {
			return nil, fmt.Errorf("failed to encode values.yaml: %w", err)
		}
		valuesStr := buf.String()
		valuesStr = replaceChartName(valuesStr, oldChartName, params.ChartName)
		valuesStr = formatValues(valuesStr)
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
			deleteYamlPath(&rootNode, []string{"backend"})
		}
		if _, ok := params.Deployments["frontend"]; !ok {
			deleteYamlPath(&rootNode, []string{"frontend"})
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
					compKebab := processor.NormalizeComponentName(compName)
					newFilename := strings.Replace(filename, baseComp, compKebab, 1)
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
			svcPort := depConfig.Service.Port
			svcTargetPort := depConfig.Service.TargetPort
			if svcPort == 0 && depConfig.Service.Ports != nil {
				if httpPort, ok := depConfig.Service.Ports["http"]; ok {
					svcPort = httpPort.Port
					svcTargetPort = httpPort.TargetPort
				}
			}
			if svcPort > 0 {
				if svcTargetPort == 0 {
					svcTargetPort = 8080 // Standard convention fallback
				}
				_ = setYamlPath(&rootNode, []string{compName, "service", "ports", "http", "port"}, svcPort)
				_ = setYamlPath(&rootNode, []string{compName, "service", "ports", "http", "targetPort"}, svcTargetPort)
				_ = setYamlPath(&rootNode, []string{compName, "startupProbe", "tcpSocket", "port"}, svcTargetPort)
				_ = setYamlPath(&rootNode, []string{compName, "livenessProbe", "tcpSocket", "port"}, svcTargetPort)
				_ = setYamlPath(&rootNode, []string{compName, "readinessProbe", "tcpSocket", "port"}, svcTargetPort)
			}
			if depConfig.Cm != nil {
				stripQuotesFromMap(depConfig.Cm)
				_ = setYamlPath(&rootNode, []string{compName, "cm"}, depConfig.Cm)
			}
			if depConfig.Secret != nil {
				stripQuotesFromMap(depConfig.Secret)
				_ = setYamlPath(&rootNode, []string{compName, "secret"}, depConfig.Secret)
			}
			if depConfig.Resources != nil {
				_ = setYamlPath(&rootNode, []string{compName, "resources"}, depConfig.Resources)
			}
			if depConfig.Hpa != nil {
				_ = setYamlPath(&rootNode, []string{compName, "hpa"}, depConfig.Hpa)
			}
			if depConfig.StartupProbe != nil {
				_ = setYamlPath(&rootNode, []string{compName, "startupProbe"}, depConfig.StartupProbe)
			}
			if depConfig.LivenessProbe != nil {
				_ = setYamlPath(&rootNode, []string{compName, "livenessProbe"}, depConfig.LivenessProbe)
			}
			if depConfig.ReadinessProbe != nil {
				_ = setYamlPath(&rootNode, []string{compName, "readinessProbe"}, depConfig.ReadinessProbe)
			}
			if depConfig.Affinity != nil {
				_ = setYamlPath(&rootNode, []string{compName, "affinity"}, depConfig.Affinity)
			}
			if depConfig.NodeSelector != nil {
				_ = setYamlPath(&rootNode, []string{compName, "nodeSelector"}, depConfig.NodeSelector)
			}
			if depConfig.Tolerations != nil {
				_ = setYamlPath(&rootNode, []string{compName, "tolerations"}, depConfig.Tolerations)
			}
			if depConfig.Route.Path != "" {
				_ = setYamlPath(&rootNode, []string{compName, "route", "path"}, depConfig.Route.Path)
			}
			if len(depConfig.ConnectsTo) > 0 {
				var connects []string
				for _, c := range depConfig.ConnectsTo {
					connects = append(connects, fmt.Sprintf(`{"apiVersion":"apps/v1","kind":"Deployment","name":"%s"}`, c))
				}
				_ = setYamlPath(&rootNode, []string{compName, "annotations", "app.openshift.io/connects-to"}, "["+strings.Join(connects, ",")+"]")
			}
			defaultHost, internalHost, externalHost := computeRouteHosts(params.ChartName, compName, depConfig.Route.Path, true)
			_ = setYamlPath(&rootNode, []string{compName, "route", "default", "enabled"}, depConfig.Route.Default.Enabled)
			if depConfig.Route.Default.Host != "" {
				defaultHost = depConfig.Route.Default.Host
			}
			_ = setYamlPath(&rootNode, []string{compName, "route", "default", "host"}, defaultHost)

			// Set runtime properties directly under the standard labels map
			if depConfig.Runtime != "" {
				_ = setYamlPath(&rootNode, []string{compName, "labels", "app.openshift.io/runtime"}, depConfig.Runtime)
			}
			if depConfig.RuntimeNamespace != "" {
				_ = setYamlPath(&rootNode, []string{compName, "labels", "app.openshift.io/runtime-namespace"}, depConfig.RuntimeNamespace)
			}
			if depConfig.RuntimeVersion != "" {
				_ = setYamlPath(&rootNode, []string{compName, "labels", "app.openshift.io/runtime-version"}, depConfig.RuntimeVersion)
			}
			// Add primary route annotation if supplied
			if depConfig.OverviewAppRoute != "" {
				_ = setYamlPath(&rootNode, []string{compName, "annotations", "console.alpha.openshift.io/overview-app-route"}, depConfig.OverviewAppRoute)
			}

			if depConfig.Persistence.Enabled {
				_ = setYamlPath(&rootNode, []string{compName, "persistence", "enabled"}, true)
				if depConfig.Persistence.Ephemeral {
					_ = setYamlPath(&rootNode, []string{compName, "persistence", "ephemeral"}, true)
				}
				if depConfig.Persistence.MountPath != "" {
					_ = setYamlPath(&rootNode, []string{compName, "persistence", "mountPath"}, depConfig.Persistence.MountPath)
				}
				if !depConfig.Persistence.Ephemeral {
					if depConfig.Persistence.StorageRequest != "" {
						_ = setYamlPath(&rootNode, []string{compName, "persistence", "storageRequest"}, depConfig.Persistence.StorageRequest)
					}
					_ = setYamlPath(&rootNode, []string{compName, "strategy"}, map[string]string{"type": "Recreate"})
				}
			
			}

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

			if len(depConfig.Files.Cm) > 0 {
				_ = setYamlPath(&rootNode, []string{compName, "files", "cm"}, depConfig.Files.Cm)
			}
			if len(depConfig.Files.Secret) > 0 {
				
				_ = setYamlPath(&rootNode, []string{compName, "files", "secret"}, depConfig.Files.Secret)
			}
		}

		if len(params.GlobalConfig) > 0 {
			stripQuotesFromMap(params.GlobalConfig)
			_ = setYamlPath(&rootNode, []string{"global", "cm"}, params.GlobalConfig)
		}
		if len(params.GlobalSecret) > 0 {
			stripQuotesFromMap(params.GlobalSecret)
			_ = setYamlPath(&rootNode, []string{"global", "secret"}, params.GlobalSecret)
		}

		// Re-marshal values.yaml preserving comments
		setBlockStyle(&rootNode)
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&rootNode); err != nil {
			return nil, fmt.Errorf("failed to encode values.yaml: %w", err)
		}
		valuesStr := buf.String()
		// Replace chart name inside values.yaml (e.g. in affinity matching labels)
		valuesStr = replaceChartName(valuesStr, oldChartName, params.ChartName)
		valuesStr = formatValues(valuesStr)
		outputFiles["values.yaml"] = []byte(valuesStr)
	}

	// Inject subcomponent values snippets
	var subValuesStr string
	for _, sub := range params.Subcomponents {
		subPath := fmt.Sprintf("models/subcomponents/%s/values-snippet.yaml", sub)
		if data, err := roothelmify.ModelsFS.ReadFile(subPath); err == nil {
			subValuesStr += "\n\n" + strings.ReplaceAll(string(data), "<CHART_NAME>", params.ChartName)
		}
	}
	if len(subValuesStr) > 0 {
		if valuesData, ok := outputFiles["values.yaml"]; ok {
			outputFiles["values.yaml"] = []byte(string(valuesData) + subValuesStr)
		}
	}

	if chartData, ok := outputFiles["Chart.yaml"]; ok {
		var chartNode yaml.Node
		if err := yaml.Unmarshal(chartData, &chartNode); err == nil {
			if params.DevRepoURL != "" {
				_ = setYamlPath(&chartNode, []string{"sources"}, []string{params.DevRepoURL})
			}
			cv := config.GlobalEnvConfig.ChartVersion
			_ = setYamlPath(&chartNode, []string{"version"}, cv)
			_ = setYamlPath(&chartNode, []string{"appVersion"}, cv)

			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&chartNode); err == nil {
				outputFiles["Chart.yaml"] = buf.Bytes()
			}
		}
	}

	basePath = "models/single"
	if params.Type == "multi" {
		basePath = "models/multi"
	}
	caData, err := roothelmify.ModelsFS.ReadFile(filepath.Join(basePath, "values-ca.yaml"))
	if err == nil {
		if valuesData, ok := outputFiles["values.yaml"]; ok {
			var values helmify.Values
			if err := yaml.Unmarshal(valuesData, &values); err == nil {
				mergedCa, err := mergeDevValues(caData, params.ChartName, values)
				if err == nil {
					outputFiles["values-ca.yaml"] = mergedCa
				}
			}
		}
	}

	outputFiles[".gitlab-ci.yml"] = roothelmify.GitLabCI

	return outputFiles, nil
}

func formatValues(valuesStr string) string {
	blocks := []string{"imagePullSecrets:", "replicas:", "labels:", "annotations:", "cm:", "secret:", "vso:", "files:", "resources:", "route:", "service:", "persistence:", "startupProbe:", "livenessProbe:", "readinessProbe:", "strategy:", "terminationGracePeriodSeconds:", "nodeSelector:", "tolerations:", "affinity:"}
	for _, block := range blocks {
		r := regexp.MustCompile(`(?m)^([^\n#]+)\n(\s+` + block + `)`)
		valuesStr = r.ReplaceAllString(valuesStr, "$1\n\n$2")
	}
	return valuesStr
}

func replaceChartName(content string, oldChartName, newChartName string) string {
	res := strings.ReplaceAll(content, oldChartName, newChartName)
	res = strings.ReplaceAll(res, "chart-model-single", newChartName)
	res = strings.ReplaceAll(res, "chart-model-multi", newChartName)
	res = strings.ReplaceAll(res, "chart-model", newChartName)
	return res
}

func replaceComponent(content string, oldComp, newComp string) string {
	newCompKebab := processor.NormalizeComponentName(newComp)
	repls := []struct{ old, new string }{
		{"chart-model-multi.fullname\" . }}-" + oldComp, "chart-model-multi.fullname\" . }}-" + newCompKebab},
		{"-" + oldComp + "-cm", "-" + newCompKebab + "-cm"},
		{"-" + oldComp + "-secret", "-" + newCompKebab + "-secret"},
		{"component: " + oldComp, "component: " + newCompKebab},
		{"name: " + oldComp, "name: " + newCompKebab},
		{"cm-" + oldComp + ".yaml", "cm-" + newCompKebab + ".yaml"},
		{"secret-" + oldComp + ".yaml", "secret-" + newCompKebab + ".yaml"},
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

func deleteYamlPath(node *yaml.Node, path []string) {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		deleteYamlPath(node.Content[0], path)
		return
	}
	if node.Kind != yaml.MappingNode || len(path) == 0 {
		return
	}
	key := path[0]
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			if len(path) == 1 {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return
			}
			deleteYamlPath(node.Content[i+1], path[1:])
			return
		}
	}
}

func setBlockStyle(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		node.Style &= ^yaml.FlowStyle
	}
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, "\n") {
		node.Style = yaml.LiteralStyle
	}
	for _, child := range node.Content {
		setBlockStyle(child)
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
				setBlockStyle(insertValNode)
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
		setBlockStyle(insertValNode)
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

func stripQuotesFromMap(m map[string]string) {
	for k, v := range m {
		val := strings.TrimSpace(v)
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) || (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			if len(val) >= 2 {
				m[k] = val[1 : len(val)-1]
			}
		}
	}
}
