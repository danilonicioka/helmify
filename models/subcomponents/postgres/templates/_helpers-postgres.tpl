{{/*
Postgres labels
*/}}
{{- define "<CHART_NAME>.postgres.labels" -}}
{{ include "<CHART_NAME>.labels" . }}
app.kubernetes.io/component: {{ include "<CHART_NAME>.fullname" . }}-postgres
{{- with .Values.postgres.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Postgres annotations
*/}}
{{- define "<CHART_NAME>.postgres.annotations" -}}
{{- end }}

{{/*
Postgres selector labels
*/}}
{{- define "<CHART_NAME>.postgres.selectorLabels" -}}
{{ include "<CHART_NAME>.selectorLabels" . }}
app.kubernetes.io/component: {{ include "<CHART_NAME>.fullname" . }}-postgres
{{- end }}
