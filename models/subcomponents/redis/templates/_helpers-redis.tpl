{{/*
Redis labels
*/}}
{{- define "<CHART_NAME>.redis.labels" -}}
{{ include "<CHART_NAME>.labels" . }}
app.kubernetes.io/component: {{ include "<CHART_NAME>.fullname" . }}-redis
{{- with .Values.redis.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Redis annotations
*/}}
{{- define "<CHART_NAME>.redis.annotations" -}}
{{- end }}

{{/*
Redis selector labels
*/}}
{{- define "<CHART_NAME>.redis.selectorLabels" -}}
{{ include "<CHART_NAME>.selectorLabels" . }}
app.kubernetes.io/component: {{ include "<CHART_NAME>.fullname" . }}-redis
{{- end }}
