{{- define "opskeeper.name" -}}opskeeper{{- end }}
{{- define "opskeeper.fullname" -}}{{ printf "%s-opskeeper" .Release.Name | trunc 63 | trimSuffix "-" }}{{- end }}
{{- define "opskeeper.labels" -}}
app.kubernetes.io/name: {{ include "opskeeper.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}
{{- define "opskeeper.selectorLabels" -}}
app.kubernetes.io/name: {{ include "opskeeper.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- define "opskeeper.image" -}}
{{- if .Values.image.digest -}}
{{ printf "%s@%s" .Values.image.repository .Values.image.digest }}
{{- else -}}
{{ printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end -}}
{{- end }}
{{- define "opskeeper.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
fsGroup: 65532
seccompProfile:
  type: RuntimeDefault
{{- end }}
{{- define "opskeeper.containerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: ["ALL"]
{{- end }}
{{- define "opskeeper.commonEnv" -}}
- name: OPSK_ENVIRONMENT
  value: production
- name: OPSK_LOG_FORMAT
  value: json
- name: OPSK_LOG_HEALTH_IGNORE
  value: {{ .Values.logHealthIgnore | quote }}
- name: OPSK_BASE_PATH
  value: {{ .Values.basePath | quote }}
- name: OPSK_COOKIE_SECURE
  value: "true"
- name: OPSK_TRUSTED_PROXIES
  value: {{ .Values.trustedProxies | quote }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ .Values.otelExporterEndpoint | quote }}
- name: OPSK_OPERATION_SUBMITTER_ENABLED
  value: {{ .Values.operation.enabled | quote }}
- name: OPSK_OPERATION_RUNNER_IMAGE
  value: {{ default (include "opskeeper.image" .) .Values.operation.runnerImage | quote }}
{{- end }}
