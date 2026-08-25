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
Api-specific labels
*/}}
{{- define "chart-model-multi.api.labels" -}}
{{ include "chart-model-multi.labels" . }}
app.kubernetes.io/component: {{ include "chart-model-multi.fullname" . }}-api
{{- with .Values.api.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Api-specific annotations
*/}}
{{- define "chart-model-multi.api.annotations" -}}
{{- with .Values.api.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
App-specific labels
*/}}
{{- define "chart-model-multi.app.labels" -}}
{{ include "chart-model-multi.labels" . }}
app.kubernetes.io/component: {{ include "chart-model-multi.fullname" . }}-app
{{- with .Values.app.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
App-specific annotations
*/}}
{{- define "chart-model-multi.app.annotations" -}}
{{- with .Values.app.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
Api-specific selector labels
*/}}
{{- define "chart-model-multi.api.selectorLabels" -}}
{{ include "chart-model-multi.selectorLabels" . }}
app.kubernetes.io/component: {{ include "chart-model-multi.fullname" . }}-api
{{- end }}

{{/*
App-specific selector labels
*/}}
{{- define "chart-model-multi.app.selectorLabels" -}}
{{ include "chart-model-multi.selectorLabels" . }}
app.kubernetes.io/component: {{ include "chart-model-multi.fullname" . }}-app
{{- end }}