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

num_reqs = 0    # use with care

logger = logging.getLogger("logger")
logger.setLevel(logging.INFO)
handler = logging.StreamHandler()
handler.setLevel(logging.INFO)
formatter = logging.Formatter('[%(asctime)s] [%(process)s] [%(levelname)s] [%(funcName)s - %(lineno)d]  %(message)s')
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
        logger.error("Bad response, aborting: {} - {} ({})".format(r.code, r.error, r.request_time))


def start_gunicorn(nworkers=4):
    cmd = ['gunicorn', '-w', str(nworkers), '-b', '0.0.0.0:5000', 'app:app']
    p = subprocess.Popen(cmd, shell=False)
    return p


def send_batch(nreqs):
    global num_reqs
    global p_uid

    http_client = httpclient.AsyncHTTPClient(max_clients=4)
    for _ in range(nreqs):
        num_reqs += 1
        endpoint = "foo" if num_reqs % 2 == 0 else "bar"
        url = "http://localhost:5000/" + endpoint + '?' + p_uid
        http_client.fetch(url, handle, method='GET', connect_timeout=90, request_timeout=120)

    logger.info("Starting tornado I/O loop")
    ioloop.IOLoop.instance().start()


class Gunicorn:

    def __init__(self, nworkers):
        self.w = nworkers

    def __enter__(self):
        self.p = start_gunicorn(self.w)
        time.sleep(3)
        logger.info("Gunicorn ready {}".format(self.p.pid))

    def __exit__(self, *args):
        logger.info("Flushing queues")
        self.p.terminate()
        time.sleep(10)
        logger.info("Gunicorn terminated")


def load_test(nworkers, nreqs):
    with Gunicorn(nworkers):
        send_batch(nreqs)


def check(index_name, size, it):
    count = size * it
    es.indices.refresh(index_name)

    def anomaly(x): return x > 100000 or x < 1  # 100000 = 0.1 sec

    err = "queried for {}, expected {}, got {}"
    for (doc_type, c) in [('transaction', count), ('trace', count * 2)]:
        rs = es.count(index=index_name, body=es_query("processor.event", doc_type))
        assert rs['count'] == c, err.format(doc_type, c, rs)

    for (trace_name, c) in [('app.foo', count / 2), ('app.bar', count / 2), ('transaction', count)]:
        rs = es.count(index=index_name, body=es_query("trace.name", trace_name))
        assert rs['count'] == c, err.format(trace_name, c, rs)

    for transaction_name in ['GET /foo', 'GET /bar']:
        rs = es.count(index=index_name, body=es_query("transaction.name.keyword", transaction_name))
        assert rs['count'] == count / 2, err.format(transaction_name, count / 2, rs)

    transactions_query = es_query("processor.event", "transaction")
    transaction_dict = {}
    for hit in lookup(es.search(index, body=transactions_query), 'hits', 'hits'):

        transaction = lookup(hit, '_source', 'transaction')
        duration = lookup(transaction, 'duration', 'us')

        transaction_dict[transaction['id']] = (transaction['name'], duration)

        assert not anomaly(duration), duration

        timestamp = datetime.strptime(lookup(hit, '_source', '@timestamp'), '%Y-%m-%dT%H:%M:%S.%fZ')
        assert datetime.utcnow() - timedelta(minutes=it) < timestamp < datetime.utcnow(), \
            "{} is too far of {} ".format(timestamp, datetime.utcnow())

        assert transaction['result'] == '200', transaction['result']
        assert transaction['type'] == 'web.flask', transaction['type']

        context = lookup(hit, '_source', 'context')
        assert context['request']['url']['search'] == p_uid, \
            "{} not in context {}".format(p_uid, context)
        assert lookup(context, 'app', 'language', 'name') == 'python', context
        assert lookup(context, 'app', 'name') == 'test-app', context
        agent_version = __import__('pkg_resources').get_distribution('elastic-apm').version
        assert lookup(context, 'app', 'agent') == {'version': agent_version, 'name': 'elasticapm-python'}, \
            "agent version {} not found in context {}".format(agent_version, context)
        assert lookup(context, 'app', 'framework', 'name') == 'flask', context
        flask_version = __import__('pkg_resources').get_distribution('flask').version
        assert lookup(context, 'app', 'framework', 'version') == flask_version, \
            "flask version {} not found in context {}".format(flask_version, context)
        assert context['tags'] == {}, context

        assert hit['_source']['processor'] == {'name': 'transaction', 'event': 'transaction'}

    traces_query = es_query("processor.event", "trace")
    for hit in lookup(es.search(index, body=traces_query), 'hits', 'hits'):
        context = lookup(hit, '_source', 'context')
        assert lookup(context, 'app', 'name') == 'test-app', context
        agent_version = __import__('pkg_resources').get_distribution('elastic-apm').version
        assert lookup(context, 'app', 'agent') == {'version': agent_version, 'name': 'elasticapm-python'}, \
            "agent version {} not found in context {}".format(agent_version, context)

        trace = lookup(hit, '_source', 'trace')
        assert trace.get('parent', 0) == 0, trace['parent']

        start = lookup(trace, 'start', 'us')
        assert not anomaly(start), start

        duration = lookup(trace, 'duration', 'us')
        assert not anomaly(duration), duration

        transaction_name, transaction_duration = transaction_dict[trace['transaction_id']]
        assert duration < transaction_duration * 10, \
            "trace duration {} is more than 10X bigger than transaction duration{}".format(duration, transaction_duration)

        stacktrace = trace['stacktrace']
        assert 15 < len(stacktrace) < 30, \
            "number of frames not expected, got {}, but this assertion might be too strict".format(len(stacktrace))

        fns = [frame['function'] for frame in stacktrace]
        assert all(fns), fns
        for attr in ['abs_path', 'line', 'module', 'filename']:
            assert all(frame[attr] for frame in stacktrace)

        if trace['name'] == 'app.bar':
            assert transaction_name == 'GET /bar', transaction_name
            assert trace['id'] == 1
            assert 'bar_route' in fns
        elif trace['name'] == 'app.foo':
            assert transaction_name == 'GET /foo', transaction_name
            assert trace['id'] == 1
            assert 'foo_route' in fns
        elif trace['name'] != 'transaction':
            assert trace['id'] == 0
            assert False, "trace name not expected {}".format(trace['name'])


def lookup(d, *keys):
    d1 = copy.deepcopy(d)
    for k in keys:
        d1 = d1[k]
    return d1


def es_query(field, val):
    return {"query": {"term": {field: val}}}


def reset():
    f = '../../../../_meta/kibana/default/index-pattern/apmserver.json'
    with open(f) as meta:
        d = json.load(meta)
        ver = d['version']

    index_name = "apm-server-{}-{}".format(ver, time.strftime('%Y.%m.%d'))
    logger.info("Deleting index of the day {}".format(index_name))
    es.indices.delete(index=index_name, ignore=[400, 404])
    return index_name


if __name__ == '__main__':
    # todo make these input-able
    iters = 10
    size = 1000
    workers = 4

    index = reset()

    for it in range(1, iters + 1):
        logger.info("Sending batch {} / {}".format(it, iters))
        load_test(workers, size)
        check(index, size, it)
        logger.info("So far so good...")

    logger.info("ALL DONE")
