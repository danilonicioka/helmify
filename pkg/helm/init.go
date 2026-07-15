package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

const helmIgnore = `# Patterns to ignore when building packages.
# This supports shell glob matching, relative path matching, and
# negation (prefixed with !). Only one pattern per line.
.DS_Store
# Common VCS dirs
.git/
.gitignore
.bzr/
.bzrignore
.hg/
.hgignore
.svn/
# Common backup files
*.swp
*.bak
*.tmp
*.orig
*~
# Various IDEs
.project
.idea/
*.tmproj
.vscode/
`

const defaultHelpers = `{{/*
Expand the name of the chart.
*/}}
{{- define "<CHARTNAME>.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "<CHARTNAME>.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "<CHARTNAME>.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "<CHARTNAME>.labels" -}}
helm.sh/chart: {{ include "<CHARTNAME>.chart" . }}
{{ include "<CHARTNAME>.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "<CHARTNAME>.selectorLabels" -}}
app.kubernetes.io/name: {{ include "<CHARTNAME>.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
API labels
*/}}
{{- define "<CHARTNAME>.api.labels" -}}
{{ include "<CHARTNAME>.labels" . }}
app.kubernetes.io/component: {{ include "<CHARTNAME>.fullname" . }}-api
{{- end }}

{{/*
API selector labels
*/}}
{{- define "<CHARTNAME>.api.selectorLabels" -}}
{{ include "<CHARTNAME>.selectorLabels" . }}
app.kubernetes.io/component: {{ include "<CHARTNAME>.fullname" . }}-api
{{- end }}

{{/*
APP labels
*/}}
{{- define "<CHARTNAME>.app.labels" -}}
{{ include "<CHARTNAME>.labels" . }}
app.kubernetes.io/component: {{ include "<CHARTNAME>.fullname" . }}-app
{{- end }}

{{/*
APP selector labels
*/}}
{{- define "<CHARTNAME>.app.selectorLabels" -}}
{{ include "<CHARTNAME>.selectorLabels" . }}
app.kubernetes.io/component: {{ include "<CHARTNAME>.fullname" . }}-app
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "<CHARTNAME>.serviceAccountName" -}}
{{- $default := (include "<CHARTNAME>.fullname" .) }}
{{- with .Values.serviceAccount }}
{{- if .create }}
{{- default $default .name }}
{{- else }}
{{- default "default" .name }}
{{- end }}
{{- end }}
{{- end }}
`

const globalConfigMapTempl = `{{- if and .Values.global .Values.global.cm (not (empty .Values.global.cm)) -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "<CHARTNAME>.fullname" . }}-global
  labels:
    {{- include "<CHARTNAME>.labels" . | nindent 4 }}
data:
{{- range $key, $val := .Values.global.cm }}
  {{ $key }}: {{ $val | quote }}
{{- end }}
{{- end }}
`
const defaultChartfile = `apiVersion: v2
name: %s
description: A standardized, production-grade model Helm chart for Tribunal de Justica do Para (TJPA) applications.
type: application
version: 1.0.0
appVersion: "1.0.0"
`

const certManagerDependencies = `
dependencies:
  - name: cert-manager
    repository: https://charts.jetstack.io
    condition: certmanager.enabled
    alias: certmanager
    version: %q
`

var chartName = regexp.MustCompile("^[a-zA-Z0-9._-]+$")

const maxChartNameLength = 250

// initChartDir - creates Helm chart structure in chartName directory if not presented.
func initChartDir(chartDir, chartName string, crd bool, certManagerAsSubchart bool, certManagerVersion string) error {
	if err := validateChartName(chartName); err != nil {
		return err
	}

	cDir := filepath.Join(chartDir, chartName)
	_, err := os.Stat(filepath.Join(cDir, "Chart.yaml"))
	if os.IsNotExist(err) {
		return createCommonFiles(chartDir, chartName, crd, certManagerAsSubchart, certManagerVersion)
	}
	logrus.Info("Skip creating Chart skeleton: Chart.yaml already exists.")
	return err
}

func validateChartName(name string) error {
	if name == "" || len(name) > maxChartNameLength {
		return fmt.Errorf("chart name must be between 1 and %d characters", maxChartNameLength)
	}
	if !chartName.MatchString(name) {
		return fmt.Errorf("chart name must match the regular expression %q", chartName.String())
	}
	return nil
}

func createCommonFiles(chartDir, chartName string, crd bool, certManagerAsSubchart bool, certManagerVersion string) error {
	cDir := filepath.Join(chartDir, chartName)
	err := os.MkdirAll(filepath.Join(cDir, "templates"), 0750)
	if err != nil {
		return fmt.Errorf("%w: unable create chart/templates dir", err)
	}
	if crd {
		err = os.MkdirAll(filepath.Join(cDir, "crds"), 0750)
		if err != nil {
			return fmt.Errorf("%w: unable create crds dir", err)
		}
	}
	createFile := func(content []byte, path ...string) {
		if err != nil {
			return
		}
		file := filepath.Join(path...)
		err = os.WriteFile(file, content, 0640)
		if err == nil {
			logrus.WithField("file", file).Info("created")
		}
	}
	createFile(chartYAML(chartName, certManagerAsSubchart, certManagerVersion), cDir, "Chart.yaml")
	createFile([]byte(helmIgnore), cDir, ".helmignore")
	createFile(helpersYAML(chartName), cDir, "templates", "_helpers.tpl")
	createFile(globalConfigMapYAML(chartName), cDir, "templates", "cm-global.yaml")
	return err
}

func globalConfigMapYAML(chartName string) []byte {
	return []byte(strings.ReplaceAll(globalConfigMapTempl, "<CHARTNAME>", chartName))
}

func chartYAML(appName string, certManagerAsSubchart bool, certManagerVersion string) []byte {
	chartFile := defaultChartfile
	if certManagerAsSubchart {
		chartFile += fmt.Sprintf(certManagerDependencies, certManagerVersion)
	}
	return []byte(fmt.Sprintf(chartFile, appName))
}

func helpersYAML(chartName string) []byte {
	return []byte(strings.ReplaceAll(defaultHelpers, "<CHARTNAME>", chartName))
}
