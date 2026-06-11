package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/arttor/helmify/pkg/cluster"
	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"
	"gopkg.in/yaml.v3"
)

// MemoryOutput captures the generated Helm chart in memory.
// It implements the helmify.Output interface.
type MemoryOutput struct {
	Files      map[string][]byte
	DevRepoURL string
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
		"fullnameOverride": chartName,
		"dnsResolver":     "dns-default.openshift-dns.svc.cluster.local",
		"global": map[string]interface{}{
			"TZ":                        "America/Belem",
			"KUBERNETES_CLUSTER_DOMAIN": cluster.DefaultDomain,
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

	// Generate component-specific ConfigMaps and Secrets for components that have variables but no templates generated yet
	for key, val := range values {
		compMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		compKebab := processor.NormalizeComponentName(key)
		if _, hasCm := compMap["cm"]; hasCm {
			cmFilename := "cm-" + compKebab + ".yaml"
			if _, exists := files[cmFilename]; !exists {
				cmContent := fmt.Sprintf(compCmTemplate, key, chartName, compKebab)
				m.Files[filepath.Join("templates", cmFilename)] = []byte(cmContent)
			}
		}
		if _, hasSecret := compMap["secret"]; hasSecret {
			secretFilename := "secret-" + compKebab + ".yaml"
			if _, exists := files[secretFilename]; !exists {
				secretContent := fmt.Sprintf(compSecretTemplate, key, chartName, compKebab)
				m.Files[filepath.Join("templates", secretFilename)] = []byte(secretContent)
			}
		}
	}

	// Write values.yaml to memory
	if certManagerAsSubchart {
		_, _ = values.Add(certManagerInstallCRD, "certmanager", "installCRDs")
		_, _ = values.Add(true, "certmanager", "enabled")
	}
	res, err := marshalOrdered(values)
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

	return nil
}

// ToTarGz bundles the captured files into a tar.gz stream.
func (m *MemoryOutput) ToTarGz(chartName string, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range m.Files {
		// All files should be nested inside a directory named "chart"
		path := filepath.Join("chart", name)
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
