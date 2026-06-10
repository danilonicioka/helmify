package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arttor/helmify/pkg/cluster"
	"github.com/arttor/helmify/pkg/helmify"
	"github.com/arttor/helmify/pkg/processor"

	"github.com/sirupsen/logrus"

	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	compCmTemplate = `{{- if and .Values.%[1]s .Values.%[1]s.cm -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "%[2]s.fullname" . }}-%[3]s-cm
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
data:
{{- range $key, $val := .Values.%[1]s.cm }}
  {{ $key }}: {{ $val | quote }}
{{- end }}
{{- end }}
`
	compSecretTemplate = `{{- if and .Values.%[1]s .Values.%[1]s.secret -}}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "%[2]s.fullname" . }}-%[3]s-secrets
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
type: Opaque
data:
{{- range $key, $val := .Values.%[1]s.secret }}
  {{ $key }}: {{ $val | b64enc | quote }}
{{- end }}
{{- end }}
`
)

// NewOutput creates interface to dump processed input to filesystem in Helm chart format.
func NewOutput() helmify.Output {
	return &output{}
}

type output struct{}

// Create a helm chart in the current directory:
// chartName/
//
//	├── .helmignore   	# Contains patterns to ignore when packaging Helm charts.
//	├── Chart.yaml    	# Information about your chart
//	├── values.yaml   	# The default values for your templates
//	└── templates/    	# The template files
//	    └── _helpers.tp   # Helm default template partials
//
// Overwrites existing values.yaml and templates in templates dir on every run.
func (o output) Create(chartDir, chartName string, crd bool, certManagerAsSubchart bool, certManagerVersion string, certManagerInstallCRD bool, templates []helmify.Template, filenames []string) error {
	err := initChartDir(chartDir, chartName, crd, certManagerAsSubchart, certManagerVersion)
	if err != nil {
		return err
	}
	// group templates into files
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
		err = values.Merge(template.Values())
		if err != nil {
			return err
		}
	}
	cDir := filepath.Join(chartDir, chartName)
	for filename, tpls := range files {
		err = overwriteTemplateFile(filename, cDir, crd, tpls)
		if err != nil {
			return err
		}
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
				err = os.WriteFile(filepath.Join(cDir, "templates", cmFilename), []byte(cmContent), 0600)
				if err != nil {
					return fmt.Errorf("%w: unable to write %s", err, cmFilename)
				}
			}
		}
		if _, hasSecret := compMap["secret"]; hasSecret {
			secretFilename := "secret-" + compKebab + ".yaml"
			if _, exists := files[secretFilename]; !exists {
				secretContent := fmt.Sprintf(compSecretTemplate, key, chartName, compKebab)
				err = os.WriteFile(filepath.Join(cDir, "templates", secretFilename), []byte(secretContent), 0600)
				if err != nil {
					return fmt.Errorf("%w: unable to write %s", err, secretFilename)
				}
			}
		}
	}

	err = overwriteValuesFile(cDir, values, certManagerAsSubchart, certManagerInstallCRD)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(cDir, "templates", "cm-global.yaml"), globalConfigMapYAML(chartName), 0600)
	if err != nil {
		return fmt.Errorf("%w: unable to write cm-global.yaml", err)
	}
	return nil
}

func overwriteTemplateFile(filename, chartDir string, crd bool, templates []helmify.Template) error {
	// pull in crd-dir setting and siphon crds into folder
	var subdir string
	if strings.Contains(filename, "crd") && crd {
		subdir = "crds"
		// create "crds" if not exists
		if _, err := os.Stat(filepath.Join(chartDir, "crds")); os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Join(chartDir, "crds"), 0750)
			if err != nil {
				return fmt.Errorf("%w: unable create crds dir", err)
			}
		}
	} else {
		subdir = "templates"
	}
	file := filepath.Join(chartDir, subdir, filename)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("%w: unable to open %s", err, file)
	}
	defer f.Close()
	for i, t := range templates {
		logrus.WithField("file", file).Debug("writing a template into")
		err = t.Write(f)
		if err != nil {
			return fmt.Errorf("%w: unable to write into %s", err, file)
		}
		if i != len(templates)-1 {
			_, err = f.Write([]byte("\n---\n"))
			if err != nil {
				return fmt.Errorf("%w: unable to write into %s", err, file)
			}
		}
	}
	if len(templates) != 0 {
		_, err = f.Write([]byte("\n"))
		if err != nil {
			return fmt.Errorf("%w: unable to write newline into %s", err, file)
		}
	}
	logrus.WithField("file", file).Info("overwritten")
	return nil
}

func overwriteValuesFile(chartDir string, values helmify.Values, certManagerAsSubchart bool, certManagerInstallCRD bool) error {
	if certManagerAsSubchart {
		_, err := values.Add(certManagerInstallCRD, "certmanager", "installCRDs")
		if err != nil {
			return fmt.Errorf("%w: unable to add cert-manager.installCRDs", err)
		}

		_, err = values.Add(true, "certmanager", "enabled")
		if err != nil {
			return fmt.Errorf("%w: unable to add cert-manager.enabled", err)
		}
	}
	// Use custom marshaler to preserve desired logical ordering
	res, err := marshalOrdered(values)
	if err != nil {
		return fmt.Errorf("%w: unable to write marshal values.yaml", err)
	}

	file := filepath.Join(chartDir, "values.yaml")
	err = os.WriteFile(file, res, 0600)
	if err != nil {
		return fmt.Errorf("%w: unable to write values.yaml", err)
	}
	logrus.WithField("file", file).Info("overwritten")

	fileDev := filepath.Join(chartDir, "values-ca.yaml")
	var valuesNode yaml.Node
	if err := yaml.Unmarshal(res, &valuesNode); err == nil {
		if devVals, err := generateDevValues(&valuesNode); err == nil {
			err = os.WriteFile(fileDev, devVals, 0600)
			if err != nil {
				return fmt.Errorf("%w: unable to write values-ca.yaml", err)
			}
			logrus.WithField("file", fileDev).Info("overwritten")
		}
	}

	return nil
}

func marshalOrdered(v interface{}) ([]byte, error) {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	node := toNode(v, 0, "")
	err := enc.Encode(node)
	res := b.String()
	res = strings.ReplaceAll(res, "\n\n  # helmify-newline\n", "\n\n")
	res = strings.ReplaceAll(res, "\n  # helmify-newline\n", "\n\n")
	return []byte(res), err
}

func toNode(v interface{}, depth int, path string) *yaml.Node {
	switch val := v.(type) {
	case map[string]interface{}:
		content := make([]*yaml.Node, 0, len(val)*2)
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			pi := getPriority(keys[i], val[keys[i]])
			pj := getPriority(keys[j], val[keys[j]])
			if pi != pj {
				return pi < pj
			}
			return keys[i] < keys[j]
		})

		var prevPriority int
		for _, k := range keys {
			p := getPriority(k, val[k])
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			if depth == 1 && prevPriority != 0 && p != prevPriority {
				keyNode.HeadComment = "helmify-newline"
			}
			
			// Inject Ultra-Lean comments for empty objects
			if isEmptyMap(val[k]) {
				var customComment string
				lookupKey := k + "." + path
				if rawVal, ok := helmify.OriginalValuesRegistry.Load(lookupKey); ok {
					if strVal, ok := rawVal.(string); ok && strVal != "" {
						customComment = strVal
					}
				}

				if customComment != "" {
					keyNode.FootComment = customComment
				} else {
					switch k {
					case "strategy":
						keyNode.FootComment = "strategy:\n  type: RollingUpdate\n  rollingUpdate:\n    maxSurge: 25%\n    maxUnavailable: 0"
					case "resources":
						keyNode.FootComment = "resources:\n  limits:\n    cpu: 500m\n    memory: 512Mi\n  requests:\n    cpu: 100m\n    memory: 256Mi"
					case "startupProbe":
						keyNode.FootComment = "startupProbe:\n  tcpSocket:\n    port: 8080\n  initialDelaySeconds: 0\n  periodSeconds: 5\n  failureThreshold: 30"
					case "livenessProbe":
						keyNode.FootComment = "livenessProbe:\n  tcpSocket:\n    port: 8080\n  initialDelaySeconds: 0\n  periodSeconds: 20\n  failureThreshold: 3"
					case "readinessProbe":
						keyNode.FootComment = "readinessProbe:\n  tcpSocket:\n    port: 8080\n  initialDelaySeconds: 0\n  periodSeconds: 10\n  successThreshold: 2\n  failureThreshold: 3"
					}
				}
			}

			var nextPath string
			if path == "" {
				nextPath = k
			} else {
				nextPath = path + "." + k
			}

			content = append(content, keyNode)
			content = append(content, toNode(val[k], depth+1, nextPath))
			prevPriority = p
		}
		return &yaml.Node{Kind: yaml.MappingNode, Content: content}
	case []interface{}:
		content := make([]*yaml.Node, len(val))
		for i, item := range val {
			content[i] = toNode(item, depth+1, path)
		}
		return &yaml.Node{Kind: yaml.SequenceNode, Content: content}
	case helmify.Values:
		return toNode(map[string]interface{}(val), depth, path)
	default:
		var node yaml.Node
		b, _ := k8syaml.Marshal(val)
		_ = yaml.Unmarshal(b, &node)
		if len(node.Content) > 0 {
			return node.Content[0]
		}
		return &node
	}
}

func isEmptyMap(v interface{}) bool {
	m, ok := v.(map[string]interface{})
	return ok && len(m) == 0
}

func getPriority(key string, value interface{}) int {
	if key == "global" {
		return -4
	}
	if key == "kubernetesClusterDomain" {
		return -3
	}
	if key == "fullnameOverride" {
		return -2
	}
	if key == "dnsResolver" {
		return -1
	}

	tjpaPriority := map[string]int{
		"labels":                        1,
		"image":                         2,
		"repository":                    3,
		"tag":                           4,
		"pullPolicy":                    5,
		"imagePullPolicy":               5,
		"replicas":                      6,
		"revisionHistoryLimit":          7,
		"strategy":                      8,
		"startupProbe":                  9,
		"livenessProbe":                 10,
		"readinessProbe":                11,
		"terminationGracePeriodSeconds": 12,
		"service":                       13,
		"resources":                     14,
		"route":                         15,
		"routeExt":                      16,
		"cm":                            17,
		"secret":                        18,
		"nodeSelector":                  19,
		"tolerations":                   20,
		"topologySpreadConstraints":     21,

		// Sub-keys
		"enabled":     30,
		"host":        31,
		"targetPort":  32,
		"annotations": 33,
		"tls":         34,
		"type":        40,
		"ports":       41,
	}

	if p, ok := tjpaPriority[key]; ok {
		return p
	}

	// 1. Workload (Priority 100)
	workloadKeys := map[string]bool{
		"podLabels": true, "podAnnotations": true, "podSecurityContext": true,
	}
	if workloadKeys[key] {
		return 100
	}

	// 2. Identity (Priority 101)
	if key == "serviceAccount" {
		return 101
	}

	// 5. Networking (Priority 105-107)
	if key == "ingress" {
		return 106
	}

	// Security & Extensions
	if strings.Contains(key, "role") || strings.Contains(key, "Role") {
		return 110
	}
	if key == "webhook" {
		return 111
	}
	if key == "crds" {
		return 112
	}

	return 500 // Others
}

const defaultCmTempl = `{{- range $component, $config := .Values }}
{{- if and (kindIs "map" $config) $config.cm }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "%[2]s.fullname" $ }}-{{ $component | kebabcase | lower }}-cm
  labels:
    {{- include "%[2]s.labels" $ | nindent 4 }}
data:
{{- range $key, $val := $config.cm }}
  {{ $key }}: {{ $val | quote }}
{{- end }}
{{- end }}
{{- end }}
`

const defaultSecretTempl = `{{- range $component, $config := .Values }}
{{- if and (kindIs "map" $config) $config.secret }}
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: {{ include "%[2]s.fullname" $ }}-{{ $component | kebabcase | lower }}-secret
  labels:
    {{- include "%[2]s.labels" $ | nindent 4 }}
data:
{{- range $key, $val := $config.secret }}
  {{ $key }}: {{ $val | b64enc | quote }}
{{- end }}
{{- end }}
{{- end }}
`

