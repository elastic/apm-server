# APM Integration

The APM integration installs Elasticsearch templates and Ingest Node pipelines for APM data.

### How to use this integration

When you add an APM integration to a policy, that policy will contain an `apm` input.
If a policy contains an `apm` input, any Elastic Agent(s) set up with that policy will run locally an APM Server binary.
You must configure your APM Agents to communicate with that APM Server.
Make sure to configure the APM Server `host` if it needs to be accessed from outside (eg. when running in Docker).

If you have RUM enabled, you must run APM Server centrally. Otherwise, you can run it at the edge machines.
To do so, download and enroll an Elastic Agent in the same machines where your instrumented services run.

Note that template, pipeline and ILM settings cannot be configured through this file - they are installed by the integration,
If you need additional pipelines, override ILM policies, etc; you must do it externally.

#### Namespace

When you create a policy in the Fleet UI, under "Advanced Settings" you can choose a Namespace.
It is recommended to use your service environment as namespace.
Data streams created by the APM integration will already include the service name.


## Compatibility and limitations

The APM integration requires Kibana 7.12 and Elasticsearch with basic license.
This version is experimental and has some limitations, listed bellow:

- Sourcemaps need to be uploaded to Elasticsearch directly.
- You need to create specific API keys for sourcemaps and central configuration.
- You can't use an Elastic Agent enrolled before 7.12.
- Not all settings are supported.

IMPORTANT: If you run APM Server with Elastic Agent manually in standalone mode, you must install the APM integration before ingestion starts.

## Traces

Traces are comprised of [spans and transactions](https://www.elastic.co/guide/en/apm/get-started/current/apm-data-model.html).
Traces are written to `traces-apm.*` indices.

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Traces -}}
| {{- Trim .Name | EscapeMarkdown -}} | {{- Trim .Description | EscapeMarkdown -}} | {{- Trim .Type | EscapeMarkdown -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

#### Examples

```json
{{.TransactionExample}}
```

```json
{{.SpanExample}}
```


## Metrics

Metrics include application-based metrics and some basic system metrics.
Metrics are written to `metrics-apm.app.*`, `metrics-apm.internal.*` and `metrics-apm.profiling.*` indices.

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Metrics -}}
| {{- Trim .Name | EscapeMarkdown -}} | {{- Trim .Description | EscapeMarkdown -}} | {{- Trim .Type | EscapeMarkdown -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

### Example

```json
{{.MetricsExample}}
```

## Logs

Logs are application error events.
Logs are written to `logs-apm.error.*` indices.

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Logs -}}
| {{- Trim .Name | EscapeMarkdown -}} | {{- Trim .Description | EscapeMarkdown -}} | {{- Trim .Type | EscapeMarkdown -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

### Example

```json
{{.ErrorExample}}
```
