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
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"github.com/danilonicioka/helmify/pkg/processor"
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
		if name != ".gitlab-ci.yml" {
			contentStr := string(content)
			contentStr = strings.ReplaceAll(contentStr, "{{REGISTRY}}", config.GlobalEnvConfig.Registry)
			contentStr = strings.ReplaceAll(contentStr, "{{DEFAULT_DOMAIN}}", config.GlobalEnvConfig.DefaultDomain)
			contentStr = strings.ReplaceAll(contentStr, "{{INTERNAL_DOMAIN}}", config.GlobalEnvConfig.InternalDomain)
			contentStr = strings.ReplaceAll(contentStr, "{{EXTERNAL_DOMAIN}}", config.GlobalEnvConfig.ExternalDomain)
			content = []byte(contentStr)
		}

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

	basePath := "models/universal"

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
	ChartName     string                      `json:"chartName" validate:"required"`
	Type          string                      `json:"type" validate:"required,oneof=single multi"`
	DevRepoURL    string                      `json:"devRepoUrl" validate:"required"`
	GlobalConfig  map[string]string           `json:"globalConfig"`
	GlobalSecret  map[string]string           `json:"globalSecret"`
	Deployments       map[string]DeploymentParams `json:"deployments" validate:"required,dive"`
	CronJobs          map[string]DeploymentParams `json:"cronJobs,omitempty"`
	Subcomponents     []string                    `json:"subcomponents"`
	SubcomponentsData map[string]interface{}          `json:"subcomponentsData,omitempty"`
}

// DeploymentParams represents configuration for a component deployment.
type DeploymentParams struct {
	WorkloadType     string                      `json:"workloadType,omitempty" validate:"omitempty,oneof=Deployment StatefulSet DaemonSet CronJob"`
	Schedule         string                      `json:"schedule,omitempty"`
	Suspend          *bool                       `json:"suspend,omitempty"`
	ConcurrencyPolicy string                     `json:"concurrencyPolicy,omitempty"`
	Replicas         *int                        `json:"replicas" validate:"omitempty,min=0"`
	Image            ImageParams                 `json:"image" validate:"required"`
	Command          []string                    `json:"command,omitempty"`
	Args             []string                    `json:"args,omitempty"`
	Annotations      map[string]string           `json:"annotations,omitempty"`
	Labels           map[string]string           `json:"labels,omitempty"`
	Service          ServiceParams               `json:"service"`
	Config           *ConfigParams               `json:"config,omitempty"`
	Secrets          *ConfigParams               `json:"secrets,omitempty"`
	Resources        *ResourceParams             `json:"resources,omitempty"`
	Persistence      PersistenceParams           `json:"persistence"`
	Autoscaling      *AutoscalingParams          `json:"autoscaling,omitempty"`
	Probes           *ProbesParams               `json:"probes,omitempty"`
	Scheduling       *SchedulingParams           `json:"scheduling,omitempty"`
	Strategy         map[string]interface{}      `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Route            RouteParams                 `json:"route"`
	ConnectsTo       []string                    `json:"connectsTo"`
	Runtime          string                      `json:"runtime"`
	RuntimeNamespace string                      `json:"runtimeNamespace"`
	RuntimeVersion   string                      `json:"runtimeVersion"`
	OverviewAppRoute string                      `json:"overviewAppRoute"`
	InitContainers   map[string]*SidecarParams   `json:"initContainers,omitempty"`
	ExtraContainers  map[string]*SidecarParams   `json:"extraContainers,omitempty"`
}

// SidecarPersistenceParams is a simplified persistence config for sidecars.
// Sidecars share the pod's PVC — they cannot create their own. Only enabled and mountPath are relevant.
type SidecarPersistenceParams struct {
	Enabled   bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	MountPath string `json:"mountPath,omitempty" yaml:"mountPath,omitempty"`
}

// SidecarParams holds configuration for extra and init containers
type SidecarParams struct {
	Enabled     *bool                    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Image       ImageParams              `json:"image" yaml:"image"`
	Command     []string                 `json:"command,omitempty" yaml:"command,omitempty"`
	Args        []string                 `json:"args,omitempty" yaml:"args,omitempty"`
	Config      *ConfigParams            `json:"config,omitempty" yaml:"config,omitempty"`
	Secrets     *ConfigParams            `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Service     ServiceParams            `json:"service,omitempty" yaml:"service,omitempty"`
	Resources   *ResourceParams          `json:"resources,omitempty" yaml:"resources,omitempty"`
	Probes      *ProbesParams            `json:"probes,omitempty" yaml:"probes,omitempty"`
	Persistence SidecarPersistenceParams `json:"persistence,omitempty" yaml:"persistence,omitempty"`
}

// ConfigParams holds env vars and mounted files
type ConfigParams struct {
	Env   map[string]string           `json:"env,omitempty" yaml:"env,omitempty"`
	Files map[string]CustomFileParams `json:"files,omitempty" yaml:"files,omitempty"`
}

// ProbesParams groups lifecycle probes
type ProbesParams struct {
	Startup   *ProbeParams `json:"startup,omitempty" yaml:"startup,omitempty"`
	Liveness  *ProbeParams `json:"liveness,omitempty" yaml:"liveness,omitempty"`
	Readiness *ProbeParams `json:"readiness,omitempty" yaml:"readiness,omitempty"`
}

// SchedulingParams groups node assignment rules
type SchedulingParams struct {
	NodeSelector map[string]interface{} `json:"nodeSelector,omitempty" yaml:"nodeSelector"`
	Tolerations  []interface{}          `json:"tolerations,omitempty" yaml:"tolerations"`
	Affinity     *AffinityParams        `json:"affinity,omitempty" yaml:"affinity,omitempty"`
}

// CustomFileParams defines custom file injection.
type CustomFileParams struct {
	MountPath string `json:"mountPath" yaml:"mountPath"`
	B64enc    bool   `json:"b64enc,omitempty" yaml:"b64enc,omitempty"`
	Content   string `json:"content" yaml:"content"`
}

// ResourceParams configures container resource requests and limits.
type ResourceParams struct {
	Limits   map[string]interface{} `json:"limits,omitempty"`
	Requests map[string]interface{} `json:"requests,omitempty"`
}

// ImageParams configures the container image.
type ImageParams struct {
	Repository  string                   `json:"repository" yaml:"repository" validate:"required"`
	Tag         string                   `json:"tag" yaml:"tag"`
	PullPolicy  string                   `json:"pullPolicy,omitempty" yaml:"pullPolicy,omitempty"`
	PullSecrets []map[string]interface{} `json:"pullSecrets,omitempty" yaml:"pullSecrets,omitempty"`
}

// ServiceParams configures the internal service port.
type ServiceParams struct {
	Port       *int `json:"port,omitempty" yaml:"port,omitempty"`
	Ports      map[string]struct {
		Port       int    `json:"port" yaml:"port"`
		Protocol   string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	} `json:"ports,omitempty" yaml:"ports,omitempty"`
}

func flattenMap(m map[string]interface{}, prefix []string, result map[string]interface{}) {
	for k, v := range m {
		fullPath := append(append([]string(nil), prefix...), k)
		if nested, ok := v.(map[string]interface{}); ok {
			flattenMap(nested, fullPath, result)
		} else {
			result[strings.Join(fullPath, ".")] = v
		}
	}
}

// setYamlPath updates or creates a value at the given path in a YAML AST.
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

// AutoscalingParams configures HPA or KEDA autoscaling.
type AutoscalingParams struct {
	Enabled bool        `json:"enabled" yaml:"enabled"`
	Engine  string      `json:"engine" yaml:"engine"`
	Hpa     *HpaParams  `json:"hpa,omitempty" yaml:"hpa,omitempty"`
	Keda    *KedaParams `json:"keda,omitempty" yaml:"keda,omitempty"`
}

// HpaParams enforces a strict ordering of HPA fields
type HpaParams struct {
	Enabled     bool        `json:"enabled" yaml:"enabled"`
	MinReplicas int         `json:"minReplicas,omitempty" yaml:"minReplicas,omitempty"`
	MaxReplicas int         `json:"maxReplicas,omitempty" yaml:"maxReplicas,omitempty"`
	Metrics     interface{} `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Behavior    interface{} `json:"behavior,omitempty" yaml:"behavior,omitempty"`
}

// KedaParams configures KEDA autoscaling.
type KedaParams struct {
	Enabled         bool        `json:"enabled" yaml:"enabled"`
	MinReplicas     int         `json:"minReplicas,omitempty" yaml:"minReplicas,omitempty"`
	MaxReplicas     int         `json:"maxReplicas,omitempty" yaml:"maxReplicas,omitempty"`
	PollingInterval int         `json:"pollingInterval,omitempty" yaml:"pollingInterval,omitempty"`
	CooldownPeriod  int         `json:"cooldownPeriod,omitempty" yaml:"cooldownPeriod,omitempty"`
	Triggers        interface{} `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	TriggerAuth     interface{} `json:"triggerAuth,omitempty" yaml:"triggerAuth,omitempty"`
}

// AffinityParams enforces strict ordering for Affinity fields
type AffinityParams struct {
	NodeAffinity    interface{} `json:"nodeAffinity,omitempty" yaml:"nodeAffinity,omitempty"`
	PodAffinity     interface{} `json:"podAffinity,omitempty" yaml:"podAffinity,omitempty"`
	PodAntiAffinity interface{} `json:"podAntiAffinity,omitempty" yaml:"podAntiAffinity,omitempty"`
}

// RouteParams configures routing options and paths.
type RouteParams struct {
	Path       string                    `json:"path"`
	Default    SubRouteParams            `json:"default"`
	Internal   SubRouteParams            `json:"internal"`
	External   SubRouteParams            `json:"external"`
	Additional map[string]SubRouteParams `json:"additional,omitempty" yaml:"additional,omitempty"`
}

// SubRouteParams configures route state and hostname.
type SubRouteParams struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Host    string `json:"host" yaml:"host"`
}

// PersistenceParams configures PVC persistence.
type PersistenceParams struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	Ephemeral      bool   `json:"ephemeral" yaml:"ephemeral"`
	MountPath      string `json:"mountPath" yaml:"mountPath"`
	StorageRequest string `json:"storageRequest" yaml:"storageRequest"`
}


// GenerateWizardChart reads single or multi chart templates from the embedded ModelsFS,
// applies customization overrides to values.yaml preserving comments, renames files and
// component references, and returns a map of relative file path -> file content.
func GenerateWizardChart(params WizardParams) (map[string][]byte, error) {
	if params.ChartName == "" {
		return nil, fmt.Errorf("chartName is required")
	}
	logrus.Infof("Starting GenerateWizardChart for %s", params.ChartName)

	basePath := "models/universal"

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

	oldChartName := "chart-model-multi"

	// 2. Setup the output map
	outputFiles := make(map[string][]byte)

	// Copy ALL files (templates, Chart.yaml, etc) as-is, just replacing oldChartName
	for relPath, data := range embeddedFiles {
		if relPath == "values.yaml" {
			continue // handled separately
		}
		content := replaceChartName(string(data), oldChartName, params.ChartName)
		if relPath == "Chart.yaml" && params.DevRepoURL != "" {
			content = strings.Replace(content, "sources: []", "sources:\n  - \""+params.DevRepoURL+"\"", 1)
		}
		outputFiles[relPath] = []byte(content)
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

	// 3. Process values.yaml
	valuesData := embeddedFiles["values.yaml"]
	var rootNode yaml.Node
	if err := yaml.Unmarshal(valuesData, &rootNode); err != nil {
		return nil, fmt.Errorf("failed to parse values.yaml: %w", err)
	}

	_ = setYamlPath(&rootNode, []string{"fullnameOverride"}, params.ChartName)

	// Collect and sort component keys
	var compKeys []string
	for k := range params.Deployments {
		compKeys = append(compKeys, k)
	}
	sort.Strings(compKeys)

	// Parse un-mutated values for cloning the "api" dummy block
	var origRoot yaml.Node
	if err := yaml.Unmarshal(valuesData, &origRoot); err != nil {
		return nil, fmt.Errorf("failed to parse original values.yaml for cloning: %w", err)
	}
	
	// Find components mapping inside rootNode
	var componentsMapping *yaml.Node
	if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
		topMapping := rootNode.Content[0]
		for i := 0; i < len(topMapping.Content); i += 2 {
			if topMapping.Content[i].Value == "deploys" {
				componentsMapping = topMapping.Content[i+1]
				break
			}
		}
	}
	
	if componentsMapping != nil {
		// Clear default components from rootNode's "deploys" map
		componentsMapping.Content = []*yaml.Node{}
	}

	// Find base component "api" from origRoot to use as template
	var baseNode *yaml.Node
	if origRoot.Kind == yaml.DocumentNode && len(origRoot.Content) > 0 {
		topMapping := origRoot.Content[0]
		for i := 0; i < len(topMapping.Content); i += 2 {
			if topMapping.Content[i].Value == "deploys" {
				origComponentsMapping := topMapping.Content[i+1]
				for j := 0; j < len(origComponentsMapping.Content); j += 2 {
					if origComponentsMapping.Content[j].Value == "api" {
						baseNode = origComponentsMapping.Content[j+1]
						break
					}
				}
				break
			}
		}
	}

	// Process each user component
	for _, compName := range compKeys {
		depConfig := params.Deployments[compName]

		if baseNode != nil && componentsMapping != nil {
			cloned := cloneYamlNode(baseNode)
			// Replace any nested references to 'api' (if we had any)
			replaceNodeComponent(cloned, "api", compName)
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: compName}
			componentsMapping.Content = append(componentsMapping.Content, keyNode, cloned)
		}

		appKeyPrefix := []string{"deploys", compName}
		
		if depConfig.Replicas != nil {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "replicas"), *depConfig.Replicas)
		}
		if depConfig.Service.Ports != nil && len(depConfig.Service.Ports) > 0 {
			for pName, pData := range depConfig.Service.Ports {
				_ = setYamlPath(&rootNode, append(appKeyPrefix, "service", "ports", pName, "port"), pData.Port)
				if pData.Protocol != "" && pData.Protocol != "TCP" {
					_ = setYamlPath(&rootNode, append(appKeyPrefix, "service", "ports", pName, "protocol"), pData.Protocol)
				}
			}
		} else if svcPort := depConfig.Service.Port; svcPort != nil && *svcPort > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "service", "ports", "http", "port"), *svcPort)
		}
		if depConfig.Autoscaling != nil {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "autoscaling"), depConfig.Autoscaling)
		}
		if depConfig.Probes != nil {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "probes"), depConfig.Probes)
		}
		if depConfig.Route.Path != "" {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "path"), depConfig.Route.Path)
		}
		if depConfig.Image.Repository != "" {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "image", "repository"), depConfig.Image.Repository)
		}
		if depConfig.Image.Tag != "" {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "image", "tag"), depConfig.Image.Tag)
		}
		if depConfig.Image.PullSecrets != nil && len(depConfig.Image.PullSecrets) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "image", "pullSecrets"), depConfig.Image.PullSecrets)
		}
		if depConfig.ExtraContainers != nil && len(depConfig.ExtraContainers) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "extraContainers"), depConfig.ExtraContainers)
		}
		if depConfig.InitContainers != nil && len(depConfig.InitContainers) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "initContainers"), depConfig.InitContainers)
		}
		if depConfig.Command != nil {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "command"), depConfig.Command)
		}
		if depConfig.Args != nil {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "args"), depConfig.Args)
		}
		if len(depConfig.Annotations) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "annotations"), depConfig.Annotations)
		}
		if len(depConfig.Labels) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "labels"), depConfig.Labels)
		}
		if depConfig.Config != nil {
			if depConfig.Config.Env != nil {
				stripQuotesFromMap(depConfig.Config.Env)
			}
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "config"), depConfig.Config)
		}
		if depConfig.Secrets != nil {
			if depConfig.Secrets.Env != nil {
				stripQuotesFromMap(depConfig.Secrets.Env)
			}
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "secrets"), depConfig.Secrets)
		}
		if depConfig.Resources != nil {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "resources"), depConfig.Resources)
		}
		if depConfig.Scheduling != nil {
			// Always ensure nodeSelector and tolerations are present so users know they can be set
			if depConfig.Scheduling.NodeSelector == nil {
				depConfig.Scheduling.NodeSelector = map[string]interface{}{}
			}
			if depConfig.Scheduling.Tolerations == nil {
				depConfig.Scheduling.Tolerations = []interface{}{}
			}
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "scheduling"), depConfig.Scheduling)
		}
		if depConfig.Strategy != nil && len(depConfig.Strategy) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "strategy"), depConfig.Strategy)
		}
		
		for _, sub := range params.Subcomponents {
			if sub != "cronjob" {
				depConfig.ConnectsTo = append(depConfig.ConnectsTo, params.ChartName+"-"+sub)
			}
		}
		if len(depConfig.ConnectsTo) > 0 {
			var connects []string
			for _, c := range depConfig.ConnectsTo {
				cName := c
				if !strings.HasPrefix(cName, params.ChartName+"-") && cName != params.ChartName {
					cName = params.ChartName + "-" + cName
				}
				connects = append(connects, fmt.Sprintf(`{"apiVersion":"apps/v1","kind":"Deployment","name":"%s"}`, cName))
			}
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "annotations", "app.openshift.io/connects-to"), "["+strings.Join(connects, ",")+"]")
		}
		if depConfig.Runtime != "" {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "labels", "app.openshift.io/runtime"), depConfig.Runtime)
		}
		if depConfig.OverviewAppRoute != "" {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "annotations", "console.alpha.openshift.io/overview-app-route"), depConfig.OverviewAppRoute)
		}

		if depConfig.Persistence.Enabled {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "persistence", "enabled"), true)
			if depConfig.Persistence.Ephemeral {
				_ = setYamlPath(&rootNode, append(appKeyPrefix, "persistence", "ephemeral"), true)
			}
			if depConfig.Persistence.MountPath != "" {
				_ = setYamlPath(&rootNode, append(appKeyPrefix, "persistence", "mountPath"), depConfig.Persistence.MountPath)
			}
			if !depConfig.Persistence.Ephemeral {
				if depConfig.Persistence.StorageRequest != "" {
					_ = setYamlPath(&rootNode, append(appKeyPrefix, "persistence", "storageRequest"), depConfig.Persistence.StorageRequest)
				}
				_ = setYamlPath(&rootNode, append(appKeyPrefix, "strategy"), map[string]string{"type": "Recreate"})
			}
		}

		defaultHost, internalHost, externalHost := computeRouteHosts(params.ChartName, params.ChartName, depConfig.Route.Path, false)
		_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "default", "enabled"), depConfig.Route.Default.Enabled)
		if depConfig.Route.Default.Host != "" {
			defaultHost = depConfig.Route.Default.Host
		}
		_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "default", "host"), defaultHost)
		_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "internal", "enabled"), depConfig.Route.Internal.Enabled)
		if depConfig.Route.Internal.Host != "" {
			internalHost = depConfig.Route.Internal.Host
		}
		_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "internal", "host"), internalHost)
		_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "external", "enabled"), depConfig.Route.External.Enabled)
		if depConfig.Route.External.Host != "" {
			externalHost = depConfig.Route.External.Host
		}
		_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "external", "host"), externalHost)

		if len(depConfig.Route.Additional) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "route", "additional"), depConfig.Route.Additional)
		}
		if depConfig.Config != nil && len(depConfig.Config.Files) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "config", "files"), depConfig.Config.Files)
		}
		if depConfig.Secrets != nil && len(depConfig.Secrets.Files) > 0 {
			_ = setYamlPath(&rootNode, append(appKeyPrefix, "secrets", "files"), depConfig.Secrets.Files)
		}
	}

	// 4. Set global variables
	if len(params.GlobalConfig) > 0 {
		stripQuotesFromMap(params.GlobalConfig)
		_ = setYamlPath(&rootNode, []string{"global", "config", "env"}, params.GlobalConfig)
	}
	if len(params.GlobalSecret) > 0 {
		stripQuotesFromMap(params.GlobalSecret)
		_ = setYamlPath(&rootNode, []string{"global", "secrets", "env"}, params.GlobalSecret)
	}

	// Process CronJobs properly
	if len(params.CronJobs) > 0 {
		for cName, cj := range params.CronJobs {
			cjKeyPrefix := []string{"cronjobs", cName}
			_ = setYamlPath(&rootNode, append(cjKeyPrefix, "enabled"), true)
			if cj.Schedule != "" {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "schedule"), cj.Schedule)
			}
			if cj.Image.Repository != "" {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "image", "repository"), cj.Image.Repository)
			}
			if cj.Image.Tag != "" {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "image", "tag"), cj.Image.Tag)
			}
			if cj.Command != nil {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "command"), cj.Command)
			}
			if cj.Args != nil {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "args"), cj.Args)
			}
			if cj.Config != nil {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "config"), cj.Config)
			}
			if cj.Secrets != nil {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "secrets"), cj.Secrets)
			}
			if cj.Suspend != nil {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "suspend"), *cj.Suspend)
			}
			if cj.ConcurrencyPolicy != "" {
				_ = setYamlPath(&rootNode, append(cjKeyPrefix, "concurrencyPolicy"), cj.ConcurrencyPolicy)
			}
		}
	}

	// Re-marshal
	setBlockStyle(&rootNode)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&rootNode); err != nil {
		return nil, fmt.Errorf("failed to encode values.yaml: %w", err)
	}
	valuesStr := buf.String()
	valuesStr = replaceChartName(valuesStr, oldChartName, params.ChartName)

	for _, sub := range params.Subcomponents {
		snippetPath := fmt.Sprintf("models/subcomponents/%s/values-snippet.yaml", sub)
		if data, err := roothelmify.ModelsFS.ReadFile(snippetPath); err == nil {
			snippetStr := strings.ReplaceAll(string(data), "<CHART_NAME>", params.ChartName)
			valuesStr += "\n" + snippetStr
		}
	}

	valuesStr = formatValues(valuesStr)
	outputFiles["values.yaml"] = []byte(valuesStr)

	// Process values-ca.yaml to generate matching components
	if caData, ok := embeddedFiles["values-ca.yaml"]; ok {
		var caRoot yaml.Node
		if err := yaml.Unmarshal(caData, &caRoot); err == nil {
			var caDeploys *yaml.Node
			var caBaseNode *yaml.Node

			if caRoot.Kind == yaml.DocumentNode && len(caRoot.Content) > 0 {
				topMapping := caRoot.Content[0]
				for i := 0; i < len(topMapping.Content); i += 2 {
					if topMapping.Content[i].Value == "deploys" {
						caDeploys = topMapping.Content[i+1]
						for j := 0; j < len(caDeploys.Content); j += 2 {
							if caDeploys.Content[j].Value == "api" {
								caBaseNode = caDeploys.Content[j+1]
								break
							}
						}
						break
					}
				}
			}

			if caDeploys != nil && caBaseNode != nil {
				caDeploys.Content = []*yaml.Node{}
				for _, compKey := range compKeys {
					cloneBytes, _ := yaml.Marshal(caBaseNode)
					var cloned yaml.Node
					_ = yaml.Unmarshal(cloneBytes, &cloned)

					if len(cloned.Content) > 0 {
						keyNode := &yaml.Node{
							Kind:  yaml.ScalarNode,
							Value: compKey,
						}
						caDeploys.Content = append(caDeploys.Content, keyNode, cloned.Content[0])
					}
				}

				setBlockStyle(&caRoot)
				var bufCa bytes.Buffer
				encCa := yaml.NewEncoder(&bufCa)
				encCa.SetIndent(2)
				if err := encCa.Encode(&caRoot); err == nil {
					caStr := replaceChartName(bufCa.String(), oldChartName, params.ChartName)
					outputFiles["values-ca.yaml"] = []byte(caStr)
				}
			}
		}
	}
	outputFiles[".gitlab-ci.yml"] = roothelmify.GitLabCI

	logrus.Infof("GenerateWizardChart complete for %s", params.ChartName)
	return outputFiles, nil
}

var formatBlocks = []string{"imagePullSecrets:", "replicas:", "labels:", "cm:", "secret:", "vso:", "resources:", "route:", "service:", "persistence:", "startupProbe:", "livenessProbe:", "readinessProbe:", "strategy:", "terminationGracePeriodSeconds:", "nodeSelector:", "tolerations:", "affinity:"}
var formatRegexes []*regexp.Regexp
var topologyRegex1 = regexp.MustCompile(`(?m)^\s+app\.openshift\.io/connects-to:\s`)
var topologyRegex2 = regexp.MustCompile(`(?m)^\s*# Example for OpenShift Topology View integration:\s*\n\s*# app\.openshift\.io/connects-to:.*\n`)
var sidecarExampleRegex = regexp.MustCompile(`(?m)^\s*#\s+sidecar-example:\s*\n(?:\s*#[^\n]*\n)+`)
var realExtraContainersRegex = regexp.MustCompile(`(?m)^\s+extraContainers:\s*\n\s+\S`)
var initExampleRegex = regexp.MustCompile(`(?m)^\s*#\s+init-example:\s*\n(?:\s*#[^\n]*\n)+`)
var realInitContainersRegex = regexp.MustCompile(`(?m)^\s+initContainers:\s*\n\s+\S`)
var cronjobExampleRegex = regexp.MustCompile(`(?m)^# =+\n# Cronjobs Configuration Example\n# =+\n(?:#[^\n]*\n)+`)
var realCronjobsRegex = regexp.MustCompile(`(?m)^cronjobs:\s*\n\s+\S`)

func init() {
	for _, block := range formatBlocks {
		formatRegexes = append(formatRegexes, regexp.MustCompile(`(?m)^([^\n#]+[^:\n#\s])\s*\n(\s+`+block+`)`))
	}
}

func formatValues(valuesStr string) string {
	for _, r := range formatRegexes {
		valuesStr = r.ReplaceAllString(valuesStr, "$1\n\n$2")
	}

	// Clean up commented connects-to example if the user explicitly provided one
	if topologyRegex1.MatchString(valuesStr) {
		valuesStr = topologyRegex2.ReplaceAllString(valuesStr, "")
	}

	// Clean up commented sidecar-example block if real extraContainers are already present
	if realExtraContainersRegex.MatchString(valuesStr) {
		valuesStr = sidecarExampleRegex.ReplaceAllString(valuesStr, "")
	}

	// Clean up commented init-example block if real initContainers are already present
	if realInitContainersRegex.MatchString(valuesStr) {
		valuesStr = initExampleRegex.ReplaceAllString(valuesStr, "")
	}

	// Clean up commented cronjobs example if real cronjobs are already present
	if realCronjobsRegex.MatchString(valuesStr) {
		valuesStr = cronjobExampleRegex.ReplaceAllString(valuesStr, "")
	}

	return valuesStr
}

func replaceChartName(content string, oldChartName, newChartName string) string {
	res := strings.ReplaceAll(content, oldChartName, newChartName)
	res = strings.ReplaceAll(res, "chart-model-single", newChartName)
	res = strings.ReplaceAll(res, "chart-model-multi", newChartName)
	res = strings.ReplaceAll(res, "chart-model", newChartName)
	res = strings.ReplaceAll(res, "<CHART_NAME>", newChartName)
	return res
}

func replaceComponent(content string, oldComp, newComp string) string {
	newCompKebab := processor.NormalizeComponentName(newComp)
	repls := []struct{ old, new string }{
		{"chart-model-multi.fullname\" . }}-" + oldComp, "chart-model-multi.fullname\" . }}-" + newCompKebab},
		{"chart-model-multi.fullname\" $ }}-" + oldComp, "chart-model-multi.fullname\" $ }}-" + newCompKebab},
		{"component: " + oldComp, "component: " + newCompKebab},
		{"name: " + oldComp, "name: " + newCompKebab},
		{"cm-" + oldComp + ".yaml", "cm-" + newCompKebab + ".yaml"},
		{"cm-" + oldComp + "-files.yaml", "cm-" + newCompKebab + "-files.yaml"},
		{"secret-" + oldComp + ".yaml", "secret-" + newCompKebab + ".yaml"},
		{"secret-" + oldComp + "-files.yaml", "secret-" + newCompKebab + "-files.yaml"},
		{"secret-truststore-" + oldComp + ".yaml", "secret-truststore-" + newCompKebab + ".yaml"},
		{"\"component\" \"" + oldComp + "\"", "\"component\" \"" + newComp + "\""},
		{".Values." + oldComp, ".Values." + newComp},
		{"index .Values \"" + oldComp + "\"", "index .Values \"" + newComp + "\""},
	}
	res := content
	for _, r := range repls {
		res = strings.ReplaceAll(res, r.old, r.new)
	}
	return res
}

func replaceNodeComponent(node *yaml.Node, oldComp, newComp string) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode {
		newCompKebab := processor.NormalizeComponentName(newComp)
		val := node.Value
		val = strings.ReplaceAll(val, "-"+oldComp, "-"+newCompKebab)
		val = strings.ReplaceAll(val, "/"+oldComp, "/"+newCompKebab)
		node.Value = val
	}
	for _, child := range node.Content {
		replaceNodeComponent(child, oldComp, newComp)
	}
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
		for i := 0; i < len(node.Content); i += 2 {
			kNode := node.Content[i]
			vNode := node.Content[i+1]
			if kNode.Value == "content" && vNode.Kind == yaml.ScalarNode {
				vNode.Tag = "!!str"
				vNode.Style = yaml.LiteralStyle
				if !strings.HasSuffix(vNode.Value, "\n") {
					vNode.Value += "\n"
				}
			}
		}
	}
	if node.Kind == yaml.ScalarNode {
		if node.Tag == "!!binary" || strings.Contains(node.Value, "\n") {
			node.Tag = "!!str"
			node.Style = yaml.LiteralStyle
			if !strings.HasSuffix(node.Value, "\n") {
				node.Value += "\n"
			}
		}
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
