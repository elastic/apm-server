{{- define "image_version" -}}
{{- default .Chart.AppVersion .Values.version }}
{{- end }}

{{- define "secret_token" -}}
{{- if .Release.IsInstall -}}
{{ (default (randAlphaNum 20) .Values.secret_token) }}
{{- else -}}
{{ $config := index (lookup "v1" "Secret" .Release.Namespace "apm-server-config").data "apm-server.yml" | b64dec | fromYaml }}
{{ (index $config "apm-server").auth.secret_token }}
{{- end }}
{{- end }}

{{- define "tls_certificate" -}}
{{- if .Release.IsInstall -}}
{{- $cert := genSelfSignedCert .Chart.Name (list "127.0.0.1") (list "localhost") 365 -}}
  tls.crt: {{ $cert.Cert | b64enc }}
  tls.key: {{ $cert.Key | b64enc }}
{{- else -}}
{{ $tls := (lookup "v1" "Secret" .Release.Namespace "apm-server-tls").data }}
  tls.crt: {{ index $tls "tls.crt" }}
  tls.key: {{ index $tls "tls.key" }}
{{- end -}}
{{- end -}}

{{- define "pod_spec" -}}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .Chart.Name }}
      app.kubernetes.io/version: {{ include "image_version" . }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .Chart.Name }}
        app.kubernetes.io/version: {{ include "image_version" . }}
    spec:
      containers:
      - name: apm-server
        image: "docker.elastic.co/apm/apm-server:{{ include "image_version" . }}"
        command: ['./apm-server', '-c', '/etc/apm-server/apm-server.yml', '--environment=container', '-E', 'apm-server.host=:8200']
        ports:
        - containerPort: 8200
        volumeMounts:
        - name: apm-server-config
          mountPath: /etc/apm-server
          readOnly: true
        - name: apm-server-tls
          mountPath: /usr/share/apm-server/tls
          readOnly: true
        - name: apm-server-data
          mountPath: /usr/share/apm-server/data
      volumes:
      - name: apm-server-config
        secret:
          secretName: apm-server-config
          items:
          - key: apm-server.yml
            path: apm-server.yml
      - name: apm-server-tls
        secret:
          secretName: apm-server-tls
      - name: apm-server-data
        {{- (.Values.storage).data | toYaml | nindent 8 }}
{{- end -}}
