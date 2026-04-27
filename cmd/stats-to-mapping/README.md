# stats-to-mapping

A developer tool that reads APM Server stats JSON from stdin and updates
upstream mapping files in-place so they expose those fields. Use it whenever a
new metric is added to APM Server's `/stats` endpoint and the corresponding
Elasticsearch monitoring templates, Metricbeat field definitions, and the
Elastic Agent integration package need to be brought in sync.

## Build

```shell
go build ./cmd/stats-to-mapping
```

This produces a `stats-to-mapping` binary in the current directory.

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

Each path is dispatched by basename or path suffix. Anything else is rejected.

## Supported files

| Target                                                                            | Source repo            | Mapping written                                                          |
| --------------------------------------------------------------------------------- | ---------------------- | ------------------------------------------------------------------------ |
| `monitoring-beats.json`                                                           | `elastic/elasticsearch` | Concrete-typed properties under `mappings._doc...metrics.properties`     |
| `monitoring-beats-mb.json`                                                        | `elastic/elasticsearch` | Alias view under `metrics.properties` and concrete view under `beat.stats` |
| `metricbeat/module/beat/_meta/fields.yml`                                         | `elastic/beats`         | Alias entries under `beats_stats`                                        |
| `metricbeat/module/beat/stats/_meta/fields.yml`                                   | `elastic/beats`         | Concrete-typed entries under `stats`                                     |
| `elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml`             | `elastic/integrations`  | Concrete-typed entries under `beat.stats`                                |

## Tests

```shell
go test ./cmd/stats-to-mapping
```

Tests are golden-file based. `testdata/inputs/` contains pinned snapshots of
the five upstream files; `testdata/golden/` contains the expected output of
running the tool against those inputs with `testdata/stats.json` on stdin. The
test fails on any byte-level difference and writes the actual output to a
temporary file path so the developer can `diff` it against the golden.

## Regenerating goldens

The committed goldens were produced once from the equivalent Python script
proposed in [elastic/apm-server#13638](https://github.com/elastic/apm-server/pull/13638)
(closed without merging). The Go program is intended to produce the same
output as that script, modulo a fix for a `seek(0)` writeback bug that left
stale bytes when new content was shorter than old.

To regenerate, against a checkout of that PR's script:

```shell
python3 -m venv /tmp/s2m-venv
/tmp/s2m-venv/bin/pip install ruamel.yaml
# Apply this one-line fix to the script's writeback so files are truncated:
sed -i 's|f.seek(0)$|f.seek(0); f.truncate()|g' /path/to/stats_to_mapping.py
# Stage inputs into the path layout expected by the script's dispatch:
TMP=$(mktemp -d)
mkdir -p $TMP/metricbeat/module/beat/{_meta,stats/_meta} \
         $TMP/elastic_agent/data_stream/apm_server_metrics/fields
cp testdata/inputs/monitoring-beats.json    $TMP/
cp testdata/inputs/monitoring-beats-mb.json $TMP/
cp testdata/inputs/beat-root-fields.yml     $TMP/metricbeat/module/beat/_meta/fields.yml
cp testdata/inputs/beat-stats-fields.yml    $TMP/metricbeat/module/beat/stats/_meta/fields.yml
cp testdata/inputs/ea-beat-fields.yml       $TMP/elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml
cat testdata/stats.json | /tmp/s2m-venv/bin/python3 /path/to/stats_to_mapping.py \
  $TMP/monitoring-beats.json \
  $TMP/monitoring-beats-mb.json \
  $TMP/metricbeat/module/beat/_meta/fields.yml \
  $TMP/metricbeat/module/beat/stats/_meta/fields.yml \
  $TMP/elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml
# Copy results back:
cp $TMP/monitoring-beats.json    testdata/golden/monitoring-beats.json
cp $TMP/monitoring-beats-mb.json testdata/golden/monitoring-beats-mb.json
cp $TMP/metricbeat/module/beat/_meta/fields.yml       testdata/golden/beat-root-fields.yml
cp $TMP/metricbeat/module/beat/stats/_meta/fields.yml testdata/golden/beat-stats-fields.yml
cp $TMP/elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml \
   testdata/golden/ea-beat-fields.yml
```

Goldens are committed bytes-as-truth. There is no in-repo regenerator script
or Python dependency — this section exists only to document how the
originally-committed goldens were produced, so a reviewer can audit them.
