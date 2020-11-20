# APM Integration

Lorem ipsum descriptium

## Compatibility

Dragons

## Configuration parameters

Maybe RUM?

## Traces

Lorem ipsum descriptium

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Traces -}}
| {{- Trim .Name -}} | {{- Trim .Description -}} | {{- Trim .Type -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

### Example

```json
{{.TransactionExample}}
```

```json
{{.SpanExample}}
```


## Metrics

Lorem ipsum descriptium

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Metrics -}}
| {{- Trim .Name -}} | {{- Trim .Description -}} | {{- Trim .Type -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

### Example

```json
{{.MetricsExample}}
```

## Logs

Lorem ipsum descriptium

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Logs -}}
| {{- Trim .Name -}} | {{- Trim .Description -}} | {{- Trim .Type -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

### Example

```json
{{.ErrorExample}}
```
