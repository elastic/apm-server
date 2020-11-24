# APM Integration

The APM integration installs templates and pipelines for APM data.
If a policy contains an `apm` input, any Elastic Agent(s) set up with that policy will run an APM Server binary, and bind to `localhost:8200`.
You must configure your APM Agents to communicate with that APM Server.

If you have RUM enabled, you must run APM Server centrally. Otherwise, you can run it at the edge machines.
To do so, download and enroll an Elastic Agent in the same machines where your instrumented services run.


### Compatibility and limitations

The APM integration requires Kibana 7.11 and Elasticsearch with basic license.
This version is experimental and has some limitations, listed bellow:

- Elastic Cloud is not supported.
- Standalone mode is not supported.
- If you need to customize settings for APM Server, you need to update the agent policy manually.
Look for `apm-server` in the `apm` input.
- It is not possible to change APM Server settings dynamically.
You must update the policy with any changes you need and stop the APM Server process.


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

Metrics include application-based metrics and some basic system metrics.
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

Logs are application log and error events.
Logs are written to `logs-apm.*` and `logs-apm.error.*` indices.

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
