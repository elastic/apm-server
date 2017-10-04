import argparse
import time
from tornado import ioloop, httpclient
from elasticsearch import Elasticsearch
from datetime import datetime, timedelta
import subprocess
import logging
import json
import copy
import uuid

p_uid = uuid.uuid4().hex

num_reqs = 0    # global variable to keep track of how many requests has been processed

logger = logging.getLogger("logger")
logger.setLevel(logging.INFO)
handler = logging.StreamHandler()
handler.setLevel(logging.INFO)
formatter = logging.Formatter(
    '[%(asctime)s] [%(process)s] [%(levelname)s] [%(funcName)s - %(lineno)d]  %(message)s')
handler.setFormatter(formatter)
logger.propagate = False
logger.addHandler(handler)

es = Elasticsearch(['localhost:9200'])


def handle(r):
    global num_reqs
    try:
        assert r.code == 200
        num_reqs -= 1
        if num_reqs == 0:
            logger.info("Stopping tornado I/O loop")
            ioloop.IOLoop.instance().stop()

    except AssertionError:
        num_reqs == 0
        ioloop.IOLoop.instance().stop()
        logger.error(
            "Bad response, aborting: {} - {} ({})".format(r.code, r.error, r.request_time))


def run_process(lang):
    if lang == 'node':
        cmd = ['node', '../../nodejs/express/app.js']
    elif lang == 'python':
        cmd = ['gunicorn', '-w', '4', '-b', '0.0.0.0:5000', 'app:app']
    p = subprocess.Popen(cmd)
    return p


def send_batch(nreqs, port=None):
    global num_reqs
    global p_uid

    mix_lang = port is None

    http_client = httpclient.AsyncHTTPClient(max_clients=4)
    for _ in range(nreqs):
        num_reqs += 1
        if mix_lang:
            num_reqs += 1
            url_python = "http://localhost:5000/foo"
            url_node = "http://localhost:8081/bar"
            for url in [url_node, url_python]:
                http_client.fetch(url, handle, method='GET',
                                  connect_timeout=90, request_timeout=120)
        else:
            endpoint = "foo" if num_reqs % 2 == 0 else "bar"
            url = "http://localhost:" + port + "/" + endpoint + '?' + p_uid
            http_client.fetch(url, handle, method='GET',
                              connect_timeout=90, request_timeout=120)

    logger.info("Starting tornado I/O loop")
    ioloop.IOLoop.instance().start()


class Worker:

    def __init__(self, lang):
        self.l = lang

    def __enter__(self):
        self.p = run_process(self.l)
        time.sleep(3)
        logger.info("Process ready {}".format(self.p.pid))

    def __exit__(self, *args):
        logger.info("Flushing queues")
        time.sleep(2)
        self.p.terminate()
        time.sleep(10)
        logger.info("Process terminated")


def load_test(lang, nreqs):
    with Worker(lang):
        send_batch(nreqs, str(PORTS[lang]))


def load_test_mixed_lang(nreqs):
    with Worker('python'):
        with Worker('node'):
            send_batch(nreqs)


PORTS = {'node': 8081, 'python': 5000}

EXPECTATIONS = {
    'node': {
        'url_search': '?',
        'agent_name': 'nodejs',
        'framework': 'express',
        'lang_key': 'runtime'
    },
    'python': {
        'url_search': '',
        'agent_name': 'elasticapm-python',
        'framework': 'flask',
        'lang_key': 'language'
    }
}


def check_counts(index_name, size, it):
    count = size * it

    es.indices.refresh(index_name)

    err = "queried for {}, expected {}, got {}"

    for doc_type in ['transaction', 'trace']:
        rs = es.count(index=index_name,
                      body=es_query("processor.event", doc_type))
        assert rs['count'] == count, err.format(doc_type, count, rs)

    for trace_name in ['app.foo', 'app.bar']:
        rs = es.count(index=index_name,
                      body=es_query("trace.name", trace_name))
        assert rs['count'] == count / 2, err.format(trace_name, count / 2, rs)

    for transaction_name in ['GET /foo', 'GET /bar']:
        rs = es.count(index=index_name, body=es_query(
            "transaction.name.keyword", transaction_name))
        assert rs['count'] == count / 2, \
            err.format(transaction_name, count / 2, rs)


def check_contents(lang, it):

    def anomaly(x): return x > 100000 or x < 1  # 100000 = 0.1 sec

    transactions_query = es_query("processor.event", "transaction")
    transaction_dict = {}
    for hit in lookup(es.search(index, body=transactions_query), 'hits', 'hits'):

        transaction = lookup(hit, '_source', 'transaction')
        duration = lookup(transaction, 'duration', 'us')

        transaction_dict[transaction['id']] = (transaction['name'], duration)

        assert not anomaly(duration), duration

        timestamp = datetime.strptime(lookup(hit, '_source', '@timestamp'),
                                      '%Y-%m-%dT%H:%M:%S.%fZ')
        assert datetime.utcnow() - timedelta(minutes=it) < timestamp < datetime.utcnow(), \
            "{} is too far of {} ".format(timestamp, datetime.utcnow())

        assert transaction['result'] == '200', transaction['result']
        assert transaction['type'] == 'request'

        context = lookup(hit, '_source', 'context')
        actual_search = context['request']['url']['search']
        assert actual_search == EXPECTATIONS[lang]['url_search'] + p_uid, \
            "{} not in context {}".format(p_uid, context)

        assert context['request']['method'] == "GET", context['request']['method']
        assert context['request']['url']['pathname'] in ("/foo", "/bar"), \
            context['request']['url']['pathname']
        assert context['request']['url']['hostname'] == 'localhost'

        if lang == 'node':
            assert context['response']['status_code'] == 200, context['response']['status_code']
            assert context['user'] == {}, context
            assert context['custom'] == {}, context

        actual_lang = lookup(
            context, 'app', EXPECTATIONS[lang]['lang_key'], 'name')
        assert actual_lang == lang, context
        assert lookup(context, 'app', 'name') == 'test-app', context

        actual_agent = lookup(context, 'app', 'agent', 'name')
        assert actual_agent == EXPECTATIONS[lang]['agent_name'], context

        actual_framework = lookup(context, 'app', 'framework', 'name')
        assert actual_framework == EXPECTATIONS[lang]['framework'], context

        assert context['tags'] == {}, context

        assert hit['_source']['processor'] == {'name': 'transaction',
                                               'event': 'transaction'}

    traces_query = es_query("processor.event", "trace")
    for hit in lookup(es.search(index, body=traces_query), 'hits', 'hits'):
        context = lookup(hit, '_source', 'context')
        assert lookup(context, 'app', 'name') == 'test-app', context

        trace = lookup(hit, '_source', 'trace')

        start = lookup(trace, 'start', 'us')
        assert not anomaly(start), start

        duration = lookup(trace, 'duration', 'us')
        assert not anomaly(duration), duration

        transaction_name, transaction_duration = transaction_dict[trace['transaction_id']]
        assert duration < transaction_duration * 10, \
            "trace duration {} is more than 10X bigger than transaction duration{}".format(
                duration, transaction_duration)

        stacktrace = trace['stacktrace']
        assert 15 < len(stacktrace) < 30, \
            "number of frames not expected, got {}, but this assertion might be too strict".format(
                len(stacktrace))

        fns = [frame['function'] for frame in stacktrace]
        assert all(fns), fns
        for attr in ['abs_path', 'line', 'filename']:
            assert all(
                frame.get(attr) for frame in stacktrace), stacktrace[0].keys()

        if trace['name'] == 'app.bar':
            assert transaction_name == 'GET /bar', transaction_name
            if lang == 'python':
                assert trace['id'] == 0, trace['id']
            assert 'bar_route' in fns
        elif trace['name'] == 'app.foo':
            assert transaction_name == 'GET /foo', transaction_name
            if lang == 'python':
                assert trace['id'] == 0, trace['id']
            assert 'foo_route' in fns
        else:
            assert False, "trace name not expected {}".format(trace['name'])


def check_contents_not_mixed():

    transactions_query = es_query("processor.event", "transaction")
    transaction_dict = {}
    for hit in lookup(es.search(index, body=transactions_query), 'hits', 'hits'):

        transaction = lookup(hit, '_source', 'transaction')
        runtime = lookup(hit, '_source', 'context', 'app', 'runtime', 'name')

        if transaction['name'] == 'GET /foo':
            assert runtime == 'CPython', runtime
        elif transaction['name'] == 'GET /bar':
            assert runtime == 'node', runtime
        else:
            assert False, transaction['name']

        transaction_dict[transaction['id']] = runtime

    traces_query = es_query("processor.event", "trace")
    for hit in lookup(es.search(index, body=traces_query), 'hits', 'hits'):

        agent = lookup(hit, '_source', 'context', 'app', 'agent', 'name')
        trace = lookup(hit, '_source', 'trace')
        runtime = transaction_dict[trace['transaction_id']]

        if trace['name'] == 'app.bar':
            assert runtime == 'node', runtime
            assert agent == EXPECTATIONS['node']['agent_name']
        elif trace['name'] == 'app.foo':
            assert runtime == 'CPython', runtime
            assert agent == EXPECTATIONS['python']['agent_name']
        else:
            assert False, "trace name not expected {}".format(trace['name'])


def lookup(d, *keys):
    d1 = copy.deepcopy(d)
    for k in keys:
        d1 = d1[k]
    return d1


def es_query(field, val):
    return {"query": {"term": {field: val}}}


def reset():
    f = '../../../../apm-server.template-es.json'
    with open(f) as meta:
        d = json.load(meta)
        ver = d['mappings']['doc']['_meta']['version']

    index_name = "apm-{}-{}".format(ver, time.strftime('%Y.%m.%d'))
    logger.info("Deleting index of the day {}".format(index_name))
    es.indices.delete(index=index_name, ignore=[400, 404])
    return index_name


if __name__ == '__main__':

    parser = argparse.ArgumentParser(description='Tests!')
    parser.add_argument(
        '-l', '--language', help='Either "node" or "python", defaults to both', default=None)
    parser.add_argument(
        '-s', '--size', help='Number of events to send on each iteration', default=1000)
    parser.add_argument(
        '-i', '--iterations', help='Number of iterations to do each test', default=1)

    args = parser.parse_args()

    langs = [args.language] if args.language else ['python', 'node']
    iters = int(args.iterations)
    size = int(args.size)

    for lang in langs:
        logger.info("Testing {} agent".format(lang))
        index = reset()

        for it in range(1, iters + 1):
            logger.info("Sending batch {} / {}".format(it, iters))
            load_test(lang, size)
            es.indices.refresh(index)
            check_counts(index, size, it)
            check_contents(lang, it)
            logger.info("So far so good...")

    if len(langs) > 1:
        logger.info("Testing all agents together")
        index = reset()
        for it in range(1, iters + 1):
            logger.info("Sending batch {} / {}".format(it, iters))
            load_test_mixed_lang(size)
            es.indices.refresh(index)
            check_counts(index, size * 2, it)
            check_contents_not_mixed()
            logger.info("So far so good...")

    logger.info("ALL DONE")
