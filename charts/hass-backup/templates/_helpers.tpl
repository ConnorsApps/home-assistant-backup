{{- define "hass-backup.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "hass-backup.fullname" -}}
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

{{- define "hass-backup.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "hass-backup.labels" -}}
helm.sh/chart: {{ include "hass-backup.chart" . }}
{{ include "hass-backup.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "hass-backup.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hass-backup.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "hass-backup.storageBackend" -}}
{{- $url := .Values.env.STORAGE_URL | default "s3://" -}}
{{- if hasPrefix "gs://" $url -}}gcs
{{- else if hasPrefix "file://" $url -}}file
{{- else -}}s3
{{- end -}}
{{- end }}

{{- define "hass-backup.imageTag" -}}
{{- if .Values.image.tag -}}
{{- .Values.image.tag -}}
{{- else -}}
{{- .Chart.AppVersion }}-{{ include "hass-backup.storageBackend" . }}
{{- end -}}
{{- end }}
