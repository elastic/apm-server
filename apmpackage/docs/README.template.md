# APM Integration

The APM integration installs Elasticsearch templates and Ingest Node pipelines for APM data.

### How to use this integration

When you add an APM integration to a policy, that policy will contain an `apm` input.
If a policy contains an `apm` input, any Elastic Agent(s) set up with that policy will run locally an APM Server binary.
You must configure your APM Agents to communicate with that APM Server.

If you have RUM enabled, you must run APM Server centrally. Otherwise, you can run it at the edge machines.
To do so, download and enroll an Elastic Agent in the same machines where your instrumented services run.

If you want to change the default APM Server configuration, you need to edit the `elastic-agent.yml` policy file manually.
Find the input with `type:apm` and add any settings under `apm-server` like you would normally do in `apm-server.yml`.
For instance:

```yaml
inputs:
  - id: ba928403-d7b8-4c09-adcb-d670c5eac89c
    name: apm-1
    revision: 1
    type: apm
    use_output: default
    meta:
      package:
        name: apm
        version: 0.1.0
    data_stream:
      namespace: default
    apm-server:
      rum:
        enabled: true
        event_rate.limit: 100
      secret_token: changeme
```

Note that template, pipeline and ILM settings cannot be configured through this file - Templates and pipelines are installed by the integration,
and ILM policies must be created externally. If you need additional pipelines, they must also be created externally.

#### Namespace

When you create a policy in the Fleet UI, under "Advanced Settings" you can choose a Namespace.
In future versions, data streams created by the APM integration will include the service name,
and you will be recommended to use the environment as namespace.

This version doesn't automatically use the service name, so the recommendation instead is to use
both the service name and the environment as the namespace.

### Compatibility and limitations

The APM integration requires Kibana 7.11 and Elasticsearch with basic license.
This version is experimental and has some limitations, listed bellow:

- It is not yet possible to change APM Server settings dynamically.
You must update the policy with any changes you need and restart the APM Server process.
- Sourcemap enrichment is not yet supported.
- There is no default ILM policy for traces (spans and transactions).

IMPORTANT: If you run APM Server with Elastic Agent manually in standalone mode, you must install the APM integration before ingestion starts.

### Configuration parameters

- `RUM`: Enables support for RUM monitoring. See the [documentation](https://www.elastic.co/guide/en/apm/server/current/configuration-rum.html) for details.


### Traces

Traces are comprised of [spans and transactions](https://www.elastic.co/guide/en/apm/get-started/current/apm-data-model.html).
Traces are written to `traces-apm.*` indices.

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Traces -}}
| {{- Trim .Name -}} | {{- Trim .Description -}} | {{- Trim .Type -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

#### Examples

```json
{{.TransactionExample}}
```

```json
{{.SpanExample}}
```


### Metrics

Metrics include application-based metrics, some basic system metrics and profiles.
Metrics are written to `metrics-apm.*`, `metrics-apm.internal.*` and `metrics-apm.profiling.*` indices.

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Metrics -}}
| {{- Trim .Name -}} | {{- Trim .Description -}} | {{- Trim .Type -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

#### Example

```json
{{.MetricsExample}}
```

### Logs

Logs are application error events.
Logs are written to `logs-apm.error.*` indices.

**Exported Fields**

| Field | Description | Type | ECS |
|---|---|---|:---:|
{{range .Logs -}}
| {{- Trim .Name -}} | {{- Trim .Description -}} | {{- Trim .Type -}} | {{if .IsECS}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-yes.png) {{else}} ![](https://doc-icons.s3.us-east-2.amazonaws.com/icon-no.png) {{end}} |
{{end}}

#### Example

```json
{{.ErrorExample}}
```
