"""
Utility script to get Opbeat intake data into elasticsearch. The input file should
be newline delimited JSON objects.

Usage:

    python intake_data_to_elasticsearch.py --input-file mydata.json --es-url https://my-elasticsearch-instance

Also, install python-dateutil and elasticsearch into your virtualenv
"""
import json
import uuid
import copy
import random
import datetime
import operator

import click

from dateutil.parser import parse as date_parse
from elasticsearch import Elasticsearch, helpers

TRANSACTION_DOC_VERSION = 2
TRACE_DOC_VERSION = 5


def transform(payload):
    transactions = []
    # check if we have traces. Really old python agents didn't have them
    if 'traces' not in payload:
        return []
    # check if this raw transaction has sampling info, discard if not
    if not isinstance(payload['traces'], dict):
        return []
    transaction_lookup = {
        (t['transaction'], t['timestamp']): t for t in payload['transactions']
    }
    for raw_transaction in payload['traces']['raw']:
        transaction = {}
        transaction['agent'] = payload['agent']
        transaction['opbeat_client'] = payload['opbeat_client']
        transaction['duration'] = raw_transaction[0]
        transaction['context'] = raw_transaction[-1]
        transaction['traces'] = []
        for raw_trace in sorted(raw_transaction[1:-1], key=operator.itemgetter(1)):
            trace = copy.copy(payload['traces']['groups'][raw_trace[0]])
            trace['start'] = raw_trace[1]
            trace['duration'] = raw_trace[2]
            transaction['traces'].append(trace)
        if not transaction['traces']:
            click.echo(json.dumps(raw_transaction, indent=2), err=True)
            continue
        root_trace = transaction['traces'][0]
        transaction['name'] = root_trace['transaction']
        try:
            transaction_from_group = transaction_lookup[(
                root_trace['transaction'], root_trace['timestamp'])]
            transaction['type'] = transaction_from_group['kind']
            transaction['result'] = str(transaction_from_group['result'])
        except KeyError:
            transaction['type'] = root_trace.get('transaction_kind', 'None')
            transaction['result'] = 'None'
        transaction['app_uuid'] = payload['app_uuid']
        timestamp = date_parse(root_trace['timestamp'])
        # add a random amount of seconds
        transaction['timestamp'] = timestamp + \
            datetime.timedelta(seconds=random.random() * 60)
        transactions.append(transaction)
    return transactions


def index_documents(transaction_list):
    for transaction in transaction_list:
        transaction_id = uuid.uuid4()
        yield {
            '_op_type': 'index',
            '_index': 'apm-server-6.0.0-alpha3-transactions-test-{:%Y-%m-%d}'.format(transaction['timestamp']),
            '_type': 'doc',
            '_source': {
                '_meta.version': TRANSACTION_DOC_VERSION,
                'transaction': {
                    'app_name': transaction['app_uuid'],
                    'name': transaction['name'],
                    'id': transaction_id,
                    'type': transaction['type'],
                    'result': transaction['result'],
                    'duration': int(transaction['duration'] * 1000),
                    'context': transaction['context'],
                },
                '@timestamp': transaction['timestamp'],
            }
        }
        for trace in transaction['traces']:
            body = {
                '_meta.version': TRACE_DOC_VERSION,
                'trace': {
                    'transaction_id': transaction_id,
                    'name': trace['signature'],
                    'type': trace['kind'],
                    'duration': int(trace['duration'] * 1000),
                    'parents': [str(p) for p in trace['parents']],
                    'start': int(float(trace['start']) * 1000),
                },
                '@timestamp': transaction['timestamp'] + datetime.timedelta(seconds=trace['duration'] / float(1000))
            }
            if 'extra' in trace and '_frames' in trace['extra']:
                body['trace.stacktrace'] = trace['extra']['_frames']
            yield {
                '_op_type': 'index',
                '_index': 'apm-server-6.0.0-alpha3-traces-test-{:%Y-%m-%d}'.format(transaction['timestamp']),
                '_type': 'doc',
                '_source': body
            }


@click.command()
@click.option('--input-file', help='Name of source json file or - for stdin', type=click.File('r'), default='-')
@click.option('--es-url', envvar='WRITER_ES_URL', help='URL of elasticsearch cluster')
@click.option('--batch-size', default=500, help='Size of batch to send to Elasticsearch', type=int)
def index(input_file, es_url, batch_size):
    """
    Reads transaction data in the "intake" format, transforms it to ES documents
    and writes them to an ES endpoint
    """
    es = Elasticsearch(es_url)
    transactions = []
    last_sent = datetime.datetime.now()
    for line in input_file:
        transactions += transform(json.loads(line))
        if transactions and (len(transactions) >= batch_size
                             or (datetime.datetime.now() - last_sent).total_seconds() > 5):
            click.echo('WRITER: indexing {} transactions'.format(
                min(len(transactions), batch_size)), err=True)
            helpers.bulk(es, index_documents(transactions[:batch_size]))
            transactions = transactions[batch_size:]
            last_sent = datetime.datetime.now()
    if transactions:
        click.echo('WRITER: indexing {} transactions'.format(
            len(transactions)), err=True)
        helpers.bulk(es, index_documents(transactions))


if __name__ == '__main__':
    index()
