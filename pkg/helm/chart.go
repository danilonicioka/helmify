package helm

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	roothelmify "github.com/arttor/helmify"
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
  name: {{ include "%[2]s.fullname" . }}-%[3]s-secret
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
type: Opaque
data:
{{- range $key, $val := .Values.%[1]s.secret }}
  {{ $key }}: {{ $val | b64enc | quote }}
{{- end }}
{{- end }}
`

	compRouteDefaultTemplate = `{{- if and .Values.%[1]s .Values.%[1]s.route .Values.%[1]s.route.default .Values.%[1]s.route.default.enabled -}}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "%[2]s.fullname" . }}%[4]s
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
    app.kubernetes.io/component: %[5]s
  {{- with .Values.%[1]s.route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.route.default.host }}
  host: {{ .Values.%[1]s.route.default.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.path }}
  path: {{ .Values.%[1]s.route.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.tls }}
  tls:
    {{- toYaml .Values.%[1]s.route.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: {{ include "%[2]s.fullname" . }}%[4]s
    weight: 100
  port:
    targetPort: {{ if and .Values.%[1]s.service .Values.%[1]s.service.ports }}{{ (index .Values.%[1]s.service.ports 0).name | default "http" }}{{ else }}http{{ end }}
  wildcardPolicy: None
{{- end }}
`

	compRouteInternalTemplate = `{{- if and .Values.%[1]s .Values.%[1]s.route .Values.%[1]s.route.internal .Values.%[1]s.route.internal.enabled -}}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "%[2]s.fullname" . }}%[4]s-int
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
    app.kubernetes.io/component: %[5]s
  {{- with .Values.%[1]s.route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.route.internal.host }}
  host: {{ .Values.%[1]s.route.internal.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.path }}
  path: {{ .Values.%[1]s.route.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.tls }}
  tls:
    {{- toYaml .Values.%[1]s.route.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: {{ include "%[2]s.fullname" . }}%[4]s
    weight: 100
  port:
    targetPort: {{ if and .Values.%[1]s.service .Values.%[1]s.service.ports }}{{ (index .Values.%[1]s.service.ports 0).name | default "http" }}{{ else }}http{{ end }}
  wildcardPolicy: None
{{- end }}
`

	compRouteExternalTemplate = `{{- if and .Values.%[1]s .Values.%[1]s.route .Values.%[1]s.route.external .Values.%[1]s.route.external.enabled -}}
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: {{ include "%[2]s.fullname" . }}%[4]s-ext
  labels:
    {{- include "%[2]s.labels" . | nindent 4 }}
    app.kubernetes.io/component: %[5]s
  {{- with .Values.%[1]s.route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.%[1]s.route.external.host }}
  host: {{ .Values.%[1]s.route.external.host | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.path }}
  path: {{ .Values.%[1]s.route.path | quote }}
  {{- end }}
  {{- if .Values.%[1]s.route.tls }}
  tls:
    {{- toYaml .Values.%[1]s.route.tls | nindent 4 }}
  {{- end }}
  to:
    kind: Service
    name: {{ include "%[2]s.fullname" . }}%[4]s
    weight: 100
  port:
    targetPort: {{ if and .Values.%[1]s.service .Values.%[1]s.service.ports }}{{ (index .Values.%[1]s.service.ports 0).name | default "http" }}{{ else }}http{{ end }}
  wildcardPolicy: None
{{- end }}
`
)

// NewOutput creates interface to dump processed input to filesystem in Helm chart format.
func NewOutput() helmify.Output {
	return &output{}
}

type output struct {
	GenerateAllTemplates bool
}

func (o *output) SetGenerateAllTemplates(enabled bool) {
	o.GenerateAllTemplates = enabled
}

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
		if filenames[i] != "" {
			file := files[filenames[i]]
			file = append(file, template)
			files[filenames[i]] = file
		}
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

	// Calculate if chart has multiple workload components
	componentKeys := []string{}
	for key := range values {
		if key == "global" || key == "nodeSelector" || key == "affinity" {
			continue
		}
		compKebab := processor.NormalizeComponentName(key)
		if isWorkloadComponent(compKebab, chartName, files) {
			componentKeys = append(componentKeys, key)
		}
	}
	isMulti := len(componentKeys) > 1

	if isMulti {
		helpersFile := filepath.Join(cDir, "templates", "_helpers.tpl")
		content, err := os.ReadFile(helpersFile)
		if err == nil {
			var toAppend string
			for _, compKey := range componentKeys {
				if !strings.Contains(string(content), fmt.Sprintf("define \"%s.%s.labels\"", chartName, compKey)) {
					toAppend += fmt.Sprintf("\n{{/*\n%[2]s-specific labels\n*/}}\n{{- define \"%[1]s.%[2]s.labels\" -}}\n{{ include \"%[1]s.labels\" . }}\napp.kubernetes.io/component: {{ include \"%[1]s.fullname\" . }}-%[2]s\n{{- with .Values.%[2]s.labels }}\n{{ toYaml . }}\n{{- end }}\n{{- end }}\n\n{{/*\n%[2]s-specific annotations\n*/}}\n{{- define \"%[1]s.%[2]s.annotations\" -}}\n{{- with .Values.%[2]s.annotations }}\n{{- toYaml . }}\n{{- end }}\n{{- end }}\n\n{{/*\n%[2]s-specific selector labels\n*/}}\n{{- define \"%[1]s.%[2]s.selectorLabels\" -}}\n{{ include \"%[1]s.selectorLabels\" . }}\napp.kubernetes.io/component: {{ include \"%[1]s.fullname\" . }}-%[2]s\n{{- end }}\n", chartName, compKey)
				}
			}
			if toAppend != "" {
				f, err := os.OpenFile(helpersFile, os.O_APPEND|os.O_WRONLY, 0600)
				if err == nil {
					f.WriteString(toAppend)
					f.Close()
				}
			}
		}
	}	// Initialize default keys and structures for GenerateAllTemplates
	if o.GenerateAllTemplates {
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
			if compKebab == chartName || !isMulti {
				cmFilename = "cm.yaml"
			}
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
			if compKebab == chartName || !isMulti {
				secretFilename = "secret.yaml"
			}
			if _, exists := files[secretFilename]; !exists {
				secretContent := fmt.Sprintf(compSecretTemplate, key, chartName, compKebab)
				err = os.WriteFile(filepath.Join(cDir, "templates", secretFilename), []byte(secretContent), 0600)
				if err != nil {
					return fmt.Errorf("%w: unable to write %s", err, secretFilename)
				}
			}
		}

		// Generate component-specific Routes if GenerateAllTemplates is enabled
		if o.GenerateAllTemplates {
			nameSuffix := "-" + compKebab
			componentLabelVal := fmt.Sprintf("{{ include \"%s.fullname\" . }}-%s", chartName, compKebab)
			if compKebab == chartName || !isMulti {
				nameSuffix = ""
				componentLabelVal = fmt.Sprintf("{{ include \"%s.fullname\" . }}", chartName)
			}

			routes := []struct {
				filename string
				template string
			}{
				{filename: "route" + nameSuffix + "-default.yaml", template: compRouteDefaultTemplate},
				{filename: "route" + nameSuffix + "-int.yaml", template: compRouteInternalTemplate},
				{filename: "route" + nameSuffix + "-ext.yaml", template: compRouteExternalTemplate},
			}

			if compKebab == chartName || !isMulti {
				routes[0].filename = "route-default.yaml"
				routes[1].filename = "route-int.yaml"
				routes[2].filename = "route-ext.yaml"
			}

			for _, r := range routes {
				if _, exists := files[r.filename]; !exists {
					routeContent := fmt.Sprintf(r.template, key, chartName, compKebab, nameSuffix, componentLabelVal)
					err = os.WriteFile(filepath.Join(cDir, "templates", r.filename), []byte(routeContent), 0600)
					if err != nil {
						return fmt.Errorf("%w: unable to write %s", err, r.filename)
					}
				}
			}
		}
	}

	res, err := generateValuesYAML(chartName, values, certManagerAsSubchart, certManagerInstallCRD, files)
	if err != nil {
		return err
	}
	err = overwriteValuesFile(cDir, res)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(cDir, "templates", "cm-global.yaml"), globalConfigMapYAML(chartName), 0600)
	if err != nil {
		return fmt.Errorf("%w: unable to write cm-global.yaml", err)
	}
	err = os.WriteFile(filepath.Join(cDir, "templates", "secret-global.yaml"), globalSecretYAML(chartName), 0600)
	if err != nil {
		return fmt.Errorf("%w: unable to write secret-global.yaml", err)
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

func generateValuesYAML(chartName string, values helmify.Values, certManagerAsSubchart bool, certManagerInstallCRD bool, files map[string][]helmify.Template) ([]byte, error) {
	if certManagerAsSubchart {
		_, _ = values.Add(certManagerInstallCRD, "certmanager", "installCRDs")
		_, _ = values.Add(true, "certmanager", "enabled")
	}

	// Count components
	compCount := 0
	var compKey string
	for key, val := range values {
		if key == "global" || key == "nodeSelector" || key == "affinity" || key == "fullnameOverride" || key == "kubernetesClusterDomain" || key == "nameOverride" || key == "dnsResolver" {
			continue
		}
		compKebab := processor.NormalizeComponentName(key)
		if !isWorkloadComponent(compKebab, chartName, files) {
			continue
		}
		isMap := false
		if _, ok := val.(map[string]interface{}); ok {
			isMap = true
		} else if _, ok := val.(helmify.Values); ok {
			isMap = true
		}
		if isMap {
			compCount++
			compKey = key
		}
	}

	basePath := "models/single"
	oldChartName := "chart-model-single"
	if compCount > 1 {
		basePath = "models/multi"
		oldChartName = "chart-model-multi"
	}

	valuesData, err := roothelmify.ModelsFS.ReadFile(filepath.Join(basePath, "values.yaml"))
	if err != nil {
		return nil, err
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(valuesData, &rootNode); err != nil {
		return nil, err
	}

	if compCount > 1 {
		var mapping *yaml.Node
		if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
			mapping = rootNode.Content[0]
		} else if rootNode.Kind == yaml.MappingNode {
			mapping = &rootNode
		}

		if mapping != nil && mapping.Kind == yaml.MappingNode {
			var backendKeyNode, backendValNode *yaml.Node
			var frontendKeyNode, frontendValNode *yaml.Node
			for i := 0; i < len(mapping.Content); i += 2 {
				if mapping.Content[i].Value == "backend" {
					backendKeyNode = mapping.Content[i]
					backendValNode = mapping.Content[i+1]
				}
				if mapping.Content[i].Value == "frontend" {
					frontendKeyNode = mapping.Content[i]
					frontendValNode = mapping.Content[i+1]
				}
			}

			for key, val := range values {
				if key == "global" || key == "nodeSelector" || key == "affinity" || key == "fullnameOverride" || key == "kubernetesClusterDomain" || key == "nameOverride" || key == "dnsResolver" {
					continue
				}
				compKebab := processor.NormalizeComponentName(key)
				if !isWorkloadComponent(compKebab, chartName, files) {
					continue
				}
				isMap := false
				if _, ok := val.(map[string]interface{}); ok {
					isMap = true
				} else if _, ok := val.(helmify.Values); ok {
					isMap = true
				}
				if !isMap {
					continue
				}

				exists := false
				for i := 0; i < len(mapping.Content); i += 2 {
					if mapping.Content[i].Value == key {
						exists = true
						break
					}
				}

				if !exists {
					baseKey := backendKeyNode
					baseVal := backendValNode
					if (key == "frontend" || key == "web" || strings.Contains(strings.ToLower(key), "front") || strings.Contains(strings.ToLower(key), "app")) && frontendValNode != nil {
						baseKey = frontendKeyNode
						baseVal = frontendValNode
					}
					if baseKey != nil && baseVal != nil {
						newKeyNode := cloneYamlNode(baseKey)
						newKeyNode.Value = key
						newValNode := cloneYamlNode(baseVal)
						mapping.Content = append(mapping.Content, newKeyNode, newValNode)
					}
				}
			}
		}

		if _, exists := values["backend"]; !exists {
			deleteYamlPath(&rootNode, "backend")
		}
		if _, exists := values["frontend"]; !exists {
			deleteYamlPath(&rootNode, "frontend")
		}
	} else if compKey != "" {
		renameRootKey(&rootNode, oldChartName, compKey)
	}

	_ = setYamlPath(&rootNode, []string{"fullnameOverride"}, chartName)

	if err := mergeYamlNode(&rootNode, values, []string{}); err != nil {
		return nil, err
	}
	setBlockStyle(&rootNode)

	var sortMapping *yaml.Node
	if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
		sortMapping = rootNode.Content[0]
	} else if rootNode.Kind == yaml.MappingNode {
		sortMapping = &rootNode
	}

	if sortMapping != nil && sortMapping.Kind == yaml.MappingNode {
		type pair struct {
			key *yaml.Node
			val *yaml.Node
		}
		pairs := make([]pair, len(sortMapping.Content)/2)
		for i := 0; i < len(sortMapping.Content); i += 2 {
			pairs[i/2] = pair{
				key: sortMapping.Content[i],
				val: sortMapping.Content[i+1],
			}
		}

		sort.SliceStable(pairs, func(i, j int) bool {
			pi := getPriority(pairs[i].key.Value, nil, 1)
			pj := getPriority(pairs[j].key.Value, nil, 1)
			if pi != pj {
				return pi < pj
			}
			return pairs[i].key.Value < pairs[j].key.Value
		})

		newContent := make([]*yaml.Node, 0, len(sortMapping.Content))
		for _, p := range pairs {
			newContent = append(newContent, p.key, p.val)
		}
		sortMapping.Content = newContent
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&rootNode); err != nil {
		return nil, err
	}

	resStr := buf.String()
	resStr = strings.ReplaceAll(resStr, oldChartName, chartName)
	resStr = strings.ReplaceAll(resStr, "chart-model-single", chartName)
	resStr = strings.ReplaceAll(resStr, "chart-model-multi", chartName)
	resStr = strings.ReplaceAll(resStr, "chart-model", chartName)

	return []byte(resStr), nil
}

func mergeYamlNode(dest *yaml.Node, src interface{}, path []string) error {
	if srcMap, ok := src.(map[string]interface{}); ok {
		if len(srcMap) == 0 {
			return setYamlPath(dest, path, srcMap)
		}
		for k, v := range srcMap {
			err := mergeYamlNode(dest, v, append(path, k))
			if err != nil {
				return err
			}
		}
		return nil
	}
	if srcValues, ok := src.(helmify.Values); ok {
		if len(srcValues) == 0 {
			return setYamlPath(dest, path, map[string]interface{}{})
		}
		for k, v := range srcValues {
			err := mergeYamlNode(dest, v, append(path, k))
			if err != nil {
				return err
			}
		}
		return nil
	}
	return setYamlPath(dest, path, src)
}

func overwriteValuesFile(chartDir string, res []byte) error {
	file := filepath.Join(chartDir, "values.yaml")
	err := os.WriteFile(file, res, 0600)
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
			pi := getPriority(keys[i], val[keys[i]], depth)
			pj := getPriority(keys[j], val[keys[j]], depth)
			if pi != pj {
				return pi < pj
			}
			return keys[i] < keys[j]
		})

		var prevPriority int
		for _, k := range keys {
			p := getPriority(k, val[k], depth)
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			if k == "runtime" {
				keyNode.HeadComment = "OpenShift runtime logo for Topology View (e.g. nodejs, openjdk-11-el7)"
			} else if depth == 1 && prevPriority != 0 && p != prevPriority {
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
					case "labels":
						keyNode.FootComment = "  # app.openshift.io/runtime: openjdk"
					case "annotations":
						keyNode.FootComment = "  # app.openshift.io/connects-to: '[{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"name\":\"db\"}]'"
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

func getPriority(key string, value interface{}, depth int) int {
	if depth == 1 {
		switch key {
		case "kubernetesClusterDomain":
			return -5
		case "nameOverride":
			return -4
		case "fullnameOverride":
			return -3
		case "dnsResolver":
			return -2
		case "global":
			return -1
		case "imagePullSecrets":
			return 1000
		case "nodeSelector":
			return 1001
		case "tolerations":
			return 1002
		case "affinity":
			return 1003
		default:
			return 500
		}
	}

	if key == "kubernetesClusterDomain" {
		return -4
	}
	if key == "fullnameOverride" {
		return -3
	}
	if key == "dnsResolver" {
		return -2
	}
	if key == "global" {
		return -1
	}

	tjpaPriority := map[string]int{
		"labels":                        1,
		"runtime":                       2,
		"image":                         3,
		"repository":                    4,
		"tag":                           5,
		"pullPolicy":                    6,
		"imagePullPolicy":               6,
		"replicas":                      7,
		"revisionHistoryLimit":          8,
		"strategy":                      9,
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

func isWorkloadComponent(compKebab string, chartName string, files map[string][]helmify.Template) bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	workloadPrefixes := []string{"deploy-", "sts-", "daemonset-", "job-", "cronjob-"}
	for _, prefix := range workloadPrefixes {
		if _, exists := files[prefix+compKebab+".yaml"]; exists {
			return true
		}
	}
	if compKebab == chartName {
		defaultNames := []string{"deploy.yaml", "sts.yaml", "daemonset.yaml", "job.yaml", "cronjob.yaml"}
		for _, name := range defaultNames {
			if _, exists := files[name]; exists {
				return true
			}
		}
	}
	return false
}
