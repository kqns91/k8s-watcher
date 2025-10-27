{{/*
Expand the name of the chart.
*/}}
{{- define "kube-watcher.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kube-watcher.fullname" -}}
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
{{- define "kube-watcher.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kube-watcher.labels" -}}
helm.sh/chart: {{ include "kube-watcher.chart" . }}
{{ include "kube-watcher.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kube-watcher.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-watcher.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kube-watcher.serviceAccountName" -}}
{{- if .Values.serviceAccountName }}
{{- .Values.serviceAccountName }}
{{- else }}
{{- include "kube-watcher.fullname" . }}
{{- end }}
{{- end }}

{{/*
Get the namespace
*/}}
{{- define "kube-watcher.namespace" -}}
{{- .Values.namespace | default .Release.Namespace }}
{{- end }}

{{/*
Get the Slack webhook URL
*/}}
{{- define "kube-watcher.slackWebhookUrl" -}}
{{- if .Values.slack.existingSecret }}
{{- printf "${SLACK_WEBHOOK_URL}" }}
{{- else }}
{{- .Values.slack.webhookUrl }}
{{- end }}
{{- end }}
