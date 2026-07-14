package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	roothelmify "github.com/arttor/helmify"
	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	"gopkg.in/yaml.v3"
)

// MemoryOutput captures the generated Helm chart in memory.
// It implements the helmify.Output interface.
type MemoryOutput struct {
	Files                map[string][]byte
	DevRepoURL           string
	GenerateAllTemplates bool
}

func (m *MemoryOutput) SetGenerateAllTemplates(enabled bool) {
	m.GenerateAllTemplates = enabled
}

// NewMemoryOutput creates a new MemoryOutput.
func NewMemoryOutput() *MemoryOutput {
	return &MemoryOutput{
		Files: make(map[string][]byte),
	}
}

func (m *MemoryOutput) Create(chartDir, chartName string, crd bool, certManagerAsSubchart bool, certManagerVersion string, certManagerInstallCRD bool, templates []helmify.Template, filenames []string) error {
	m.Files["Chart.yaml"] = chartYAML(chartName, certManagerAsSubchart, certManagerVersion)
	if m.DevRepoURL != "" {
		var chartNode yaml.Node
		if err := yaml.Unmarshal(m.Files["Chart.yaml"], &chartNode); err == nil {
			_ = setYamlPath(&chartNode, []string{"annotations", "tjpa.jus.br/dev-source-repo"}, m.DevRepoURL)
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&chartNode); err == nil {
				m.Files["Chart.yaml"] = buf.Bytes()
			}
		}
	}
	m.Files[".helmignore"] = []byte(helmIgnore)
	m.Files[filepath.Join("templates", "_helpers.tpl")] = helpersYAML(chartName)
	m.Files[filepath.Join("templates", "cm-global.yaml")] = globalConfigMapYAML(chartName)

	// Group templates into files
	files := map[string][]helmify.Template{}
	values := helmify.Values{
		"kubernetesClusterDomain": "cluster.local",
		"nameOverride":            "",
		"fullnameOverride":        chartName,
		"global": map[string]interface{}{
			"cm": map[string]interface{}{
				"TZ": "America/Belem",
			},
			"secret": map[string]interface{}{},
		},
	}

	for i, template := range templates {
		file := files[filenames[i]]
		file = append(file, template)
		files[filenames[i]] = file
		if err := values.Merge(template.Values()); err != nil {
			return err
		}
	}

	// Write templates to memory
	for filename, tpls := range files {
		var subdir string
		if strings.Contains(filename, "crd") && crd {
			subdir = "crds"
		} else {
			subdir = "templates"
		}

		var buf bytes.Buffer
		for i, t := range tpls {
			if err := t.Write(&buf); err != nil {
				return err
			}
			if i != len(tpls)-1 {
				buf.Write([]byte("\n---\n"))
			}
		}
		if len(tpls) != 0 {
			buf.Write([]byte("\n"))
		}
		m.Files[filepath.Join(subdir, filename)] = buf.Bytes()
	}

	// Initialize default keys and structures for GenerateAllTemplates
	if m.GenerateAllTemplates {
		for key, val := range values {
			if key == "global" || key == "nodeSelector" || key == "affinity" {
				continue
			}
			compKebab := processor.NormalizeComponentName(key)
			if !isWorkloadComponent(compKebab, chartName, files) {
				continue
			}
			compMap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}

			if _, hasCm := compMap["cm"]; !hasCm {
				compMap["cm"] = map[string]interface{}{}
			}
			if _, hasSecret := compMap["secret"]; !hasSecret {
				compMap["secret"] = map[string]interface{}{}
			}
			if _, hasRoute := compMap["route"]; !hasRoute {
				isMulti := false
				compCount := 0
				for k := range values {
					if k != "global" && k != "nodeSelector" && k != "affinity" {
						kKebab := processor.NormalizeComponentName(k)
						if isWorkloadComponent(kKebab, chartName, files) {
							compCount++
						}
					}
				}
				if compCount > 1 {
					isMulti = true
				}
				defaultHost, internalHost, externalHost := computeRouteHosts(chartName, key, "/", isMulti)
				compMap["route"] = map[string]interface{}{
					"annotations": map[string]interface{}{},
					"tls": map[string]interface{}{
						"termination": "edge",
						"insecureEdgeTerminationPolicy": "Redirect",
					},
					"path": "/",
					"default": map[string]interface{}{
						"enabled": true,
						"host":    defaultHost,
					},
					"internal": map[string]interface{}{
						"enabled": false,
						"host":    internalHost,
					},
					"external": map[string]interface{}{
						"enabled": false,
						"host":    externalHost,
					},
				}
			}
		}
	}

	// Generate component-specific ConfigMaps and Secrets for components that have variables but no templates generated yet
	for key, val := range values {
		if key == "global" || key == "nodeSelector" || key == "affinity" {
			continue
		}
		compKebab := processor.NormalizeComponentName(key)
		if !isWorkloadComponent(compKebab, chartName, files) {
			continue
		}
		compMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasCm := compMap["cm"]; hasCm {
			cmFilename := "cm-" + compKebab + ".yaml"
			if compKebab == chartName {
				cmFilename = "cm.yaml"
			}
			if _, exists := files[cmFilename]; !exists {
				cmContent := fmt.Sprintf(compCmTemplate, key, chartName, compKebab)
				m.Files[filepath.Join("templates", cmFilename)] = []byte(cmContent)
			}
		}
		if _, hasSecret := compMap["secret"]; hasSecret {
			secretFilename := "secret-" + compKebab + ".yaml"
			if compKebab == chartName {
				secretFilename = "secret.yaml"
			}
			if _, exists := files[secretFilename]; !exists {
				secretContent := fmt.Sprintf(compSecretTemplate, key, chartName, compKebab)
				m.Files[filepath.Join("templates", secretFilename)] = []byte(secretContent)
			}
		}

		// Generate component-specific Routes if GenerateAllTemplates is enabled
		if m.GenerateAllTemplates {
			nameSuffix := "-" + compKebab
			isComponent := "true"
			if compKebab == chartName {
				nameSuffix = ""
				isComponent = "false"
			}

			routes := []struct {
				filename string
				template string
			}{
				{filename: "route" + nameSuffix + "-default.yaml", template: compRouteDefaultTemplate},
				{filename: "route" + nameSuffix + "-int.yaml", template: compRouteInternalTemplate},
				{filename: "route" + nameSuffix + "-ext.yaml", template: compRouteExternalTemplate},
			}

			if compKebab == chartName {
				routes[0].filename = "route-default.yaml"
				routes[1].filename = "route-int.yaml"
				routes[2].filename = "route-ext.yaml"
			}

			for _, r := range routes {
				if _, exists := files[r.filename]; !exists {
					routeContent := fmt.Sprintf(r.template, key, chartName, compKebab, nameSuffix, isComponent)
					m.Files[filepath.Join("templates", r.filename)] = []byte(routeContent)
				}
			}
		}
	}

	// Write values.yaml to memory
	res, err := generateValuesYAML(chartName, values, certManagerAsSubchart, certManagerInstallCRD, files)
	if err != nil {
		return err
	}
	m.Files["values.yaml"] = res

	var valuesNode yaml.Node
	if err := yaml.Unmarshal(res, &valuesNode); err == nil {
		if devVals, err := generateDevValues(&valuesNode); err == nil {
			m.Files["values-ca.yaml"] = devVals
		}
	}

	m.Files[".gitlab-ci.yml"] = roothelmify.GitLabCI

	return nil
}

// ToTarGz bundles the captured files into a tar.gz stream.
func (m *MemoryOutput) ToTarGz(chartName string, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range m.Files {
		var path string
		if name == ".gitlab-ci.yml" || name == "README.md" {
			path = name
		} else {
			path = filepath.Join("chart", name)
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
