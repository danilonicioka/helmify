{{/*
Expand the name of the chart.
*/}}
{{- define "test-app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "test-app.fullname" -}}
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
{{- define "test-app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "test-app.labels" -}}
helm.sh/chart: {{ include "test-app.chart" . }}
{{ include "test-app.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- $app := index .Values .Chart.Name | default dict -}}
{{- with $app.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Annotations helper
*/}}
{{- define "test-app.annotations" -}}
{{- $app := index .Values .Chart.Name | default dict -}}
{{- with $app.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "test-app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "test-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "test-app.serviceAccountName" -}}
{{- $default := (include "test-app.fullname" .) }}
{{- with .Values.serviceAccount }}
{{- if .create }}
{{- default $default .name }}
{{- else }}
{{- default "default" .name }}
{{- end }}
{{- end }}
{{- end }}

{{/*
api-specific labels
*/}}
{{- define "test-app.api.labels" -}}
{{ include "test-app.labels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-api
{{- with .Values.api.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
api-specific annotations
*/}}
{{- define "test-app.api.annotations" -}}
{{- with .Values.api.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
api-specific selector labels
*/}}
{{- define "test-app.api.selectorLabels" -}}
{{ include "test-app.selectorLabels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-api
{{- end }}

{{/*
nginx-specific labels
*/}}
{{- define "test-app.nginx.labels" -}}
{{ include "test-app.labels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-nginx
{{- with .Values.nginx.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
nginx-specific annotations
*/}}
{{- define "test-app.nginx.annotations" -}}
{{- with .Values.nginx.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
nginx-specific selector labels
*/}}
{{- define "test-app.nginx.selectorLabels" -}}
{{ include "test-app.selectorLabels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-nginx
{{- end }}

{{/*
appEmissor-specific labels
*/}}
{{- define "test-app.appEmissor.labels" -}}
{{ include "test-app.labels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-appEmissor
{{- with .Values.appEmissor.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
appEmissor-specific annotations
*/}}
{{- define "test-app.appEmissor.annotations" -}}
{{- with .Values.appEmissor.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
appEmissor-specific selector labels
*/}}
{{- define "test-app.appEmissor.selectorLabels" -}}
{{ include "test-app.selectorLabels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-appEmissor
{{- end }}

{{/*
myapp-specific labels
*/}}
{{- define "test-app.myapp.labels" -}}
{{ include "test-app.labels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-myapp
{{- with .Values.myapp.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
myapp-specific annotations
*/}}
{{- define "test-app.myapp.annotations" -}}
{{- with .Values.myapp.annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
myapp-specific selector labels
*/}}
{{- define "test-app.myapp.selectorLabels" -}}
{{ include "test-app.selectorLabels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-myapp
{{- end }}

{{/*
app-emissor-specific labels
*/}}
{{- define "test-app.app-emissor.labels" -}}
{{ include "test-app.labels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-app-emissor
{{- with (index .Values "app-emissor").labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
app-emissor-specific annotations
*/}}
{{- define "test-app.app-emissor.annotations" -}}
{{- with (index .Values "app-emissor").annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
app-emissor-specific selector labels
*/}}
{{- define "test-app.app-emissor.selectorLabels" -}}
{{ include "test-app.selectorLabels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-app-emissor
{{- end }}

{{/*
app-specific labels
*/}}
{{- define "test-app.app.labels" -}}
{{ include "test-app.labels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-app
{{- with (index .Values "app").labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
app-specific annotations
*/}}
{{- define "test-app.app.annotations" -}}
{{- with (index .Values "app").annotations }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
app-specific selector labels
*/}}
{{- define "test-app.app.selectorLabels" -}}
{{ include "test-app.selectorLabels" . }}
app.kubernetes.io/component: {{ include "test-app.fullname" . }}-app
{{- end }}
