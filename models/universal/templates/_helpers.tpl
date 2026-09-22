{{/*
Expand the name of the chart.
*/}}
{{- define "chart-model-multi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "chart-model-multi.fullname" -}}
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
{{- define "chart-model-multi.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "chart-model-multi.labels" -}}
helm.sh/chart: {{ include "chart-model-multi.chart" . }}
{{ include "chart-model-multi.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "chart-model-multi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chart-model-multi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Component-specific selector labels
*/}}
{{- define "chart-model-multi.component.selectorLabels" -}}
{{ include "chart-model-multi.selectorLabels" .context }}
app.kubernetes.io/component: {{ include "chart-model-multi.componentname" (dict "context" .context "component" .component) }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "chart-model-multi.serviceAccountName" -}}
{{- $default := (include "chart-model-multi.fullname" .) }}
{{- with .Values.serviceAccount }}
{{- if .create }}
{{- default $default .name }}
{{- else }}
{{- default "default" .name }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Component-specific labels
*/}}
{{- define "chart-model-multi.component.labels" -}}
{{ include "chart-model-multi.labels" .context }}
app.kubernetes.io/component: {{ include "chart-model-multi.componentname" (dict "context" .context "component" .component) }}
{{- if hasKey .context.Values.deploys .component }}
{{- with (index .context.Values.deploys .component).labels }}
{{ toYaml . }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Component-specific annotations
*/}}
{{- define "chart-model-multi.component.annotations" -}}
{{- if hasKey .context.Values.deploys .component }}
{{- with (index .context.Values.deploys .component).annotations }}
{{ toYaml . }}
{{- end }}
{{- end }}
{{- end }}



{{/*
Create a default fully qualified component name.
We truncate at 63 chars because some Kubernetes name fields are limited to this.
If component name is the same as the fullname or chart name, it omits the suffix to avoid duplication (e.g. identifica-identifica).
*/}}
{{- define "chart-model-multi.componentname" -}}
{{- $fullname := include "chart-model-multi.fullname" .context -}}
{{- if or (eq .component $fullname) (eq .component .context.Chart.Name) (eq .component (default .context.Chart.Name .context.Values.nameOverride)) -}}
{{- $fullname | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" $fullname .component | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
