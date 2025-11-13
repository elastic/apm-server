import json
import pathlib
import sys

from ruamel.yaml import YAML


METRICS = ['apm-server', 'output']

def get_type(v: any) -> str:
    if isinstance(v, int):
        return "long"
    else:
        raise ValueError("unknown type")


def _convert(output_list: list[dict[str, any]], input_dict: dict[str, any], alias_prefix: str, alias: bool) -> None:
    """
    Recursively convert input_dict to output_list
    :param output_list: mutable list for output
    :param input_dict: input dict parsed from stats
    :param alias_prefix: alias prefix only relevant when alias is True
    :param alias: switch to output alias or actual type
    """
    for k, v in input_dict.items():
        next_prefix = alias_prefix + '.' + k
        if isinstance(v, dict):
            ll = []
            output_list.append({
                'name': k,
                'type': 'group',
                'fields': ll,
            })
            _convert(ll, v, next_prefix, alias)
        else:
            if alias:
                item = {
                    'name': k,
                    'type': 'alias',
                    'path': next_prefix,
                }
            else:
                item = {
                    'name': k,
                    'type': get_type(v),
                }
            output_list.append(item)


def _collapse(l: list) -> None:
    """
    Useful for YAML output
    To flatten nested fields to dotted names
    """
    for v in l:
        if v['type'] == 'group':
            _collapse(v['fields'])
            if len(v['fields']) == 1:
                vv = v['fields'][0]
                if vv['type'] == 'group':
                    continue
                elif vv['type'] == 'alias':
                    v['name'] = v['name'] + '.' + vv['name']
                    v['type'] = 'alias'
                    v['path'] = vv['path']
                    del v['fields']
                else:
                    v['name'] = v['name'] + '.' + vv['name']
                    v['type'] = vv['type']
                    del v['fields']


def _merge(src: dict[str, any], dst: dict[str, any]) -> None:
    for k, v in src.items():
        if isinstance(v, dict):
            _merge(v, dst.setdefault(k, {}))
        else:
            dst[k] = v


def _dot_to_dict(k: str, v: any) -> dict:
    prefix, sep, suffix = k.partition('.')
    if not sep:
        return {
            k: v
        }
    return {prefix: _dot_to_dict(suffix, v)}


def _nest(in_dict: dict[str, any]) -> dict[str, any]:
    """
    Useful for JSON output
    Input: dict of parsed json from stats endpoint
    To workaround dotted field names, see https://github.com/elastic/apm-server/issues/13625
    """
    out_dict: dict[str, any] = {}
    k: str
    for k, v in in_dict.copy().items():
        if isinstance(v, dict):
            v = _nest(v)
        _merge(_dot_to_dict(k, v), out_dict)
    return out_dict


def fields_yaml(stats_json: str, metric: str, alias: bool) -> list[dict[str, any]]:
    """
    # Use alias=True for metricbeat/module/beat/_meta/fields.yml
    # Use alias=False for metricbeat/module/beat/stats/_meta/fields.yml
    """
    l = []
    j = json.loads(stats_json)
    underscore_metric_name = metric.replace('-', '_')
    _convert(l, j[metric], f'beat.stats.{underscore_metric_name}', alias=alias)
    _collapse(l)
    return l


def _to_template_json(l: list[dict[str, any]]) -> dict[str, any]:
    d = {}
    for item in l:
        if item['type'] == 'alias':
            d[item['name']] = {
                'type': 'alias',
                'path': item['path'],
            }
        elif item['type'] == 'group':
            d[item['name']] = {
                'properties': _to_template_json(item['fields']),
            }
        else:
            d[item['name']] = {
                'type': item['type'],
            }
    return d


def to_template_json_properties_dict(stats_json: str, metric: str, alias: bool) -> dict[str, any]:
    l = []
    j = json.loads(stats_json)
    j = _nest(j)
    underscore_metric_name = metric.replace('-', '_')
    _convert(l, j[metric], f'beat.stats.{underscore_metric_name}', alias=alias)
    return _to_template_json(l)


def modify_monitoring_beats(path: pathlib.Path, stats_json: str) -> None:
    REPLACE_FROM = '"version": ${xpack.monitoring.template.release.version},'
    REPLACE_TO = '"version": null,'
    with path.open('r+') as f:
        j = f.read()
        j = j.replace(REPLACE_FROM, REPLACE_TO)
        beats_json = json.loads(j)
        for metric in METRICS:
            beats_json['mappings']['_doc']['properties']['beats_stats']['properties']['metrics']['properties'][
                metric] = {'properties': to_template_json_properties_dict(stats_json, metric, alias=False)}
        j = json.dumps(beats_json, indent=2)
        j = j.replace(REPLACE_TO, REPLACE_FROM)
        f.seek(0)
        f.write(j)
        f.write('\n')


def modify_monitoring_beats_mb(path: pathlib.Path, stats_json: str) -> None:
    REPLACE_FROM = '"version": ${xpack.stack.monitoring.template.release.version},'
    REPLACE_TO = '"version": null,'
    with path.open('r+') as f:
        j = f.read()
        j = j.replace(REPLACE_FROM, REPLACE_TO)
        beats_json = json.loads(j)
        for metric in METRICS:
            beats_json['template']['mappings']['properties']['beats_stats']['properties']['metrics']['properties'][
                metric] = {'properties': to_template_json_properties_dict(stats_json, metric, alias=True)}
            underscore_metric = metric.replace('-', '_')
            beats_json['template']['mappings']['properties']['beat']['properties']['stats']['properties'][
                underscore_metric] = {'properties': to_template_json_properties_dict(stats_json, metric, alias=False)}

        j = json.dumps(beats_json, indent=2)
        j = j.replace(REPLACE_TO, REPLACE_FROM)
        f.seek(0)
        f.write(j)
        f.write('\n')


def _tr(s: str) -> str:
    return '\n'.join(l[2:] for l in s.splitlines()) + '\n'  # strip leading 2 spaces


def modify_beat_root_file(path: pathlib.Path, stats_json: str) -> None:
    with path.open('r+') as f:
        yaml = YAML()
        yaml.preserve_quotes = True
        yaml.default_flow_style = False
        yaml.indent(mapping=2, sequence=4, offset=2)
        y = yaml.load(f)
        fields = y[0]['fields'][0]['fields']
        for metric in METRICS:
            to_add = {
                'name': metric,
                'type': 'group',
                'fields': fields_yaml(stats_json, metric, alias=True),
            }
            upsert_to_yaml_fields(fields, to_add)
        f.seek(0)
        yaml.dump(y, f, transform=_tr)


def modify_beat_stats_file(path: pathlib.Path, stats_json: str) -> None:
    with path.open('r+') as f:
        yaml = YAML()
        yaml.preserve_quotes = True
        yaml.default_flow_style = False
        yaml.indent(mapping=2, sequence=4, offset=2)
        y = yaml.load(f)
        fields = y[0]['fields']
        for metric in METRICS:
            underscore_metric = metric.replace('-', '_')
            to_add = {
                'name': underscore_metric,
                'type': 'group',
                'fields': fields_yaml(stats_json, metric, alias=False),
            }
            upsert_to_yaml_fields(fields, to_add)
        f.seek(0)
        yaml.dump(y, f, transform=_tr)


def modify_ea_fields_file(path: pathlib.Path, stats_json: str) -> None:
    with path.open('r+') as f:
        yaml = YAML()
        yaml.preserve_quotes = True
        yaml.default_flow_style = False
        yaml.indent(mapping=2, sequence=4, offset=2)
        y = yaml.load(f)
        for metric in METRICS:
            for top_field in y:
                if top_field['name'] != 'beat.stats':
                    continue
                fields = top_field['fields']
                underscore_metric = metric.replace('-', '_')
                to_add = {
                    'name': underscore_metric,
                    'type': 'group',
                    'fields': fields_yaml(stats_json, metric, alias=False),
                }
                upsert_to_yaml_fields(fields, to_add)
                break
            else:
                raise ValueError('beat.stats not found')
        f.seek(0)
        yaml.dump(y, f, transform=_tr)


def upsert_to_yaml_fields(fields: list[dict[str, any]], to_add: dict[str, any]) -> None:
    for i in range(len(fields)):
        if fields[i]['name'] == to_add['name']:
            fields[i] = to_add
            break
    else:  # if break was never reached
        fields.append(to_add)


def main() -> None:
    usage = (f'Usage: {sys.argv[0]} path_to_file...' +
             ("""

Path has to lead to any one of these files:
- monitoring-beats.json
- monitoring-beats-mb.json
- metricbeat/module/beat/stats/_meta/fields.yml
- metricbeat/module/beat/_meta/fields.yml
- elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml

This script reads apm-server monitoring stats json (from apm-server:5066/stats) from stdin, and modify the specified files in-place to contain the mappings.
"""))
    if len(sys.argv) < 2:
        print('Error: no file specified', file=sys.stdout)
        print(usage, file=sys.stdout)
        sys.exit(1)

    input_data = ''.join(sys.stdin.readlines())

    for path_str in sys.argv[1:]:
        path = pathlib.Path(path_str)
        if path.name == 'monitoring-beats.json':
            # https://github.com/elastic/elasticsearch/blob/main/x-pack/plugin/core/template-resources/src/main/resources/monitoring-beats.json
            modify_monitoring_beats(path, input_data)
        elif path.name == 'monitoring-beats-mb.json':
            # https://github.com/elastic/elasticsearch/blob/main/x-pack/plugin/core/template-resources/src/main/resources/monitoring-beats-mb.json
            modify_monitoring_beats_mb(path, input_data)
        elif path.resolve().match('metricbeat/module/beat/stats/_meta/fields.yml'):
            # https://github.com/elastic/beats/blob/main/metricbeat/module/beat/_meta/fields.yml
            modify_beat_stats_file(path, input_data)
        elif path.resolve().match('metricbeat/module/beat/_meta/fields.yml'):
            # https://github.com/elastic/beats/blob/main/metricbeat/module/beat/stats/_meta/fields.yml
            modify_beat_root_file(path, input_data)
        elif path.resolve().match('elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml'):
            # https://github.com/elastic/integrations/blob/main/packages/elastic_agent/data_stream/apm_server_metrics/fields/beat-fields.yml
            modify_ea_fields_file(path, input_data)
        else:
            print('Error: path does not lead to any of the expected files', file=sys.stdout)
            print(usage, file=sys.stdout)
            sys.exit(1)


if __name__ == '__main__':
    main()
