{{/*
Return chart name.
*/}}
{{- define "recluster-sync.name" -}}
recluster-sync
{{- end }}

{{/*
Return fully qualified name <release>-<chart>.
*/}}
{{- define "recluster-sync.fullname" -}}
{{ .Release.Name }}-{{ include "recluster-sync.name" . }}
{{- end }}