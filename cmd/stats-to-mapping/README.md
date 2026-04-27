# stats-to-mapping

A developer tool that reads APM Server stats JSON from stdin and updates
upstream mapping files in-place so they expose those fields. Use it whenever
a new metric is added to APM Server's `/stats` endpoint and the corresponding
Elasticsearch monitoring templates, Metricbeat field definitions, and the
Elastic Agent integration package need to be brought in sync.

## Build

```shell
go build ./cmd/stats-to-mapping
```

## Usage

Capture an APM Server stats document and pipe it into the binary along with
one or more target file paths:

```shell
curl -s http://localhost:5066/stats > apm-server.stats.json

./stats-to-mapping \
  ~/projects/elasticsearch/x-pack/plugin/core/template-resources/src/main/resources/monitoring-beats.json \
  ~/projects/elasticsearch/x-pack/plugin/core/template-resources/src/main/resources/monitoring-beats-mb.json \
  ~/projects/beats/metricbeat/module/beat/_meta/fields.yml \
  ~/projects/beats/metricbeat/module/beat/stats/_meta/fields.yml \
  ~/projects/integrations/packages/elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml \
  < apm-server.stats.json
```

Each path is dispatched by basename or path suffix. Anything else is
rejected. Modifications are in place; review the resulting `git diff` per
upstream checkout and submit per-repo PRs.

## Supported files

| Target | Source repo | Modifies |
| --- | --- | --- |
| `monitoring-beats.json` | `elastic/elasticsearch` | Concrete-typed properties under `mappings._doc.…metrics.properties` |
| `monitoring-beats-mb.json` | `elastic/elasticsearch` | Alias view under `metrics.properties` and concrete view under `beat.stats` |
| `metricbeat/module/beat/_meta/fields.yml` | `elastic/beats` | Alias entries under `beats_stats` |
| `metricbeat/module/beat/stats/_meta/fields.yml` | `elastic/beats` | Concrete-typed entries under `stats` |
| `elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml` | `elastic/integrations` | Concrete-typed entries under `beat.stats` |

## What gets touched

Both YAML and JSON modifications are byte-spliced: only the target subtree
(the `apm-server` and `output` entries inside the relevant parent) is
rewritten. Every byte outside those entries — comments, quoting, sibling
keys, the version placeholder in the JSON templates — is preserved verbatim.

Within the spliced subtree, JSON keys are emitted in alphabetical order
(stdlib `encoding/json`); YAML emission follows the upstream conventions of
those repos (4-column indent per nesting level, dashes at parent_col+2,
mapping body at dash+2).

## Tests

```shell
go test ./cmd/stats-to-mapping
```

Golden-file based. `testdata/inputs/` holds pinned snapshots of the five
upstream files; `testdata/golden/` holds the expected output. On mismatch
the test writes the actual output to a temp path and reports a `diff`
command for inspection.

To regenerate goldens after an intentional behavior change, run the tool
against the committed inputs and copy the results back into `testdata/golden/`.
