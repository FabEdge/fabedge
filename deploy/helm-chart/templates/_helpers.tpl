{{- define "connector.labels" -}}
app: {{ .Values.connector.name }}
release: {{ .Release.Name }}
{{- end -}}

{{- define "fabedgeOperator.labels" -}}
app: {{ .Values.operator.name }}
release: {{ .Release.Name }}
{{- end -}}

{{- define "cert.labels" -}}
app: {{ .Values.cert.name }}
release: {{ .Release.Name }}
{{- end -}}

{{- define "cloudAgent.labels" -}}
app: {{ .Values.cloudAgent.name }}
release: {{ .Release.Name }}
{{- end -}}

{{- define "connector.node.addresses" -}}
  {{- $foundFirst := false -}}
  {{ range $index, $node := (lookup "v1" "Node" "" "").items -}}
    {{- if hasKey $node.metadata.labels "node-role.kubernetes.io/connector" -}}
      {{- range $node.status.addresses -}}
        {{- if eq .type "InternalIP" -}}
          {{- if $foundFirst -}},{{- end -}}
          {{- .address -}}
          {{- $foundFirst = true -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end }}
