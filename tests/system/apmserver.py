from datetime import datetime, timedelta
import json
import os
import re
import shutil
import subprocess
import sys
import threading
import time
import unittest
from urllib.parse import urlparse

from elasticsearch import Elasticsearch, NotFoundError
import requests

# Add libbeat/tests/system to the import path.
output = subprocess.check_output(["go", "list", "-m", "-f", "{{.Path}} {{.Dir}}", "all"]).decode("utf-8")
beats_line = [line for line in output.splitlines() if line.startswith("github.com/elastic/beats/")][0]
beats_dir = beats_line.split(" ", 2)[1]
sys.path.append(os.path.join(beats_dir, 'libbeat', 'tests', 'system'))

from beat.beat import INTEGRATION_TESTS, TestCase, TimeoutError
from helper import wait_until
from es_helper import cleanup, default_pipelines
from es_helper import index_smap, index_span, index_error, apm_prefix
from kibana import Kibana

integration_test = unittest.skipUnless(INTEGRATION_TESTS, "integration test")
diagnostic_interval = float(os.environ.get('DIAGNOSTIC_INTERVAL', 0))


class BaseTest(TestCase):
    maxDiff = None

    def setUp(self):
        super(BaseTest, self).setUp()
        # TODO: move to Mixin and use only in tests where self.es is available
        self.setup_diagnostics()

    def setup_diagnostics(self):
        if diagnostic_interval <= 0:
            return
        self.addCleanup(self.cleanup_diagnostics)
        self.diagnostics_path = os.path.join(self.working_dir, "diagnostics")
        os.makedirs(self.diagnostics_path)
        self.running = True
        self.diagnostic_thread = threading.Thread(
            target=self.dump_diagnotics, kwargs=dict(interval=diagnostic_interval))
        self.diagnostic_thread.daemon = True
        self.diagnostic_thread.start()

    def cleanup_diagnostics(self):
        self.running = False
        self.diagnostic_thread.join(timeout=30)

    def dump_diagnotics(self, interval=2):
        while self.running:
            # TODO: use threading.Timer instead to not block tearDown
            time.sleep(interval)
            with open(os.path.join(self.diagnostics_path,
                                   datetime.now().strftime("%Y%m%d_%H%M%S") + ".hot_threads"), mode="w") as out:
                try:
                    out.write(self.es.nodes.hot_threads(threads=99999))
                except Exception as e:
                    out.write("failed to query hot threads: {}\n".format(e))

            with open(os.path.join(self.diagnostics_path,
                                   datetime.now().strftime("%Y%m%d_%H%M%S") + ".tasks"), mode="w") as out:
                try:
                    json.dump(self.es.tasks.list(), out, indent=True, sort_keys=True)
                except Exception as e:
                    out.write("failed to query tasks: {}\n".format(e))

    @classmethod
    def setUpClass(cls):
        cls.beat_name = "apm-server"
        cls.beat_path = os.path.abspath(os.path.join(
            os.path.dirname(__file__), "..", ".."))
        cls.build_path = cls._beat_path_join("build", "system-tests")
        super(BaseTest, cls).setUpClass()

    @classmethod
    def _beat_path_join(cls, *paths):
        return os.path.abspath(os.path.join(cls.beat_path, *paths))

    @staticmethod
    def get_elasticsearch_url(user="", password=""):
        """
        Returns an elasticsearch URL including username and password
        """
        host = os.getenv("ES_HOST", "localhost")

        if not user:
            user = os.getenv("ES_USER", "apm_server_user")
        if not password:
            password = os.getenv("ES_PASS", "changeme")

        if user and password:
            host = user + ":" + password + "@" + host

        return "http://{host}:{port}".format(
            host=host,
            port=os.getenv("ES_PORT", "9200"),
        )

    @staticmethod
    def get_kibana_url(user="", password=""):
        """
        Returns kibana URL including username and password
        """
        host = os.getenv("KIBANA_HOST", "localhost")

        if not user:
            user = os.getenv("KIBANA_USER", "apm_user_ro")
        if not password:
            password = os.getenv("KIBANA_PASS", "changeme")

        if user and password:
            host = user + ":" + password + "@" + host

        return "http://{host}:{port}".format(
            host=host,
            port=os.getenv("KIBANA_PORT", "5601"),
        )

    def get_payload_path(self, name):
        return self.get_testdata_path('intake-v2', name)

    def get_testdata_path(self, *names):
        return self._beat_path_join('testdata', *names)

    def get_payload(self, name):
        with open(self.get_payload_path(name)) as f:
            return f.read()

    def get_error_payload_path(self):
        return self.get_payload_path("errors_2.ndjson")

    def get_transaction_payload_path(self):
        return self.get_payload_path("transactions.ndjson")

    def get_metricset_payload_path(self):
        return self.get_payload_path("metricsets.ndjson")

    def get_event_payload(self, name="events.ndjson"):
        return self.get_payload(name)

    def ilm_index(self, index):
        return "{}-000001".format(index)


class ServerBaseTest(BaseTest):
    config_overrides = {}

    host = "http://localhost:8200"
    root_url = "{}/".format(host)
    agent_config_url = "{}/{}".format(host, "config/v1/agents")
    rum_agent_config_url = "{}/{}".format(host, "config/v1/rum/agents")
    intake_url = "{}/{}".format(host, 'intake/v2/events')
    rum_intake_url = "{}/{}".format(host, 'intake/v2/rum/events')
    sourcemap_url = "{}/{}".format(host, 'assets/v1/sourcemaps')
    expvar_url = "{}/{}".format(host, 'debug/vars')

    jaeger_grpc_host = "localhost:14250"
    jaeger_http_host = "localhost:14268"
    jaeger_http_url = "http://{}/{}".format(jaeger_http_host, 'api/traces')

    def config(self):
        cfg = {"ssl_enabled": "false",
               "queue_flush": 0,
               "jaeger_grpc_enabled": "true",
               "jaeger_grpc_host": self.jaeger_grpc_host,
               "jaeger_http_enabled": "true",
               "jaeger_http_host": self.jaeger_http_host,
               "path": os.path.abspath(self.working_dir) + "/log/*"}
        cfg.update(self.config_overrides)
        return cfg

    def setUp(self):
        super(ServerBaseTest, self).setUp()

        # Copy ingest pipeline definition to home directory of the test.
        # The pipeline definition is expected to be at a specific location
        # relative to the home dir. This ensures that the file can be loaded
        # for all installations (deb, tar, ..).
        pipeline_dir = os.path.join("ingest", "pipeline")
        pipeline_def = os.path.join(pipeline_dir, "definition.json")
        target_dir = os.path.join(self.working_dir, pipeline_dir)
        if not os.path.exists(target_dir):
            os.makedirs(target_dir)
        shutil.copy(self._beat_path_join(pipeline_def), target_dir)

        self.render_config_template(**self.config())
        self.start_proc()
        self.wait_until_started()

    def start_proc(self):
        self.apmserver_proc = self.start_beat(**self.start_args())
        self.addCleanup(self.stop_proc)

    def stop_proc(self):
        self.apmserver_proc.check_kill_and_wait()

    def start_args(self):
        return {}

    def wait_until_started(self):
        wait_until(lambda: self.log_contains("Starting apm-server"), name="apm-server started")

    def assert_no_logged_warnings(self, suppress=None):
        """
        Assert that the log file contains no ERR or WARN lines.
        """
        if suppress == None:
            suppress = []

        # Jenkins runs as a Windows service and when Jenkins executes theses
        # tests the Beat is confused since it thinks it is running as a service.
        winErr = "ERR Error: The service process could not connect to the service controller."

        corsWarn = "WARN\t.*CORS related setting .* Consider more restrictive setting for production use."

        suppress = suppress + ["WARN EXPERIMENTAL", "WARN BETA", "WARN.*deprecated", winErr, corsWarn]
        log = self.get_log()
        for s in suppress:
            log = re.sub(s, "", log)
        self.assertNotRegex(log, "ERR|WARN")

    def request_intake(self, data=None, url=None, headers=None):
        if not url:
            url = self.intake_url
        if data is None:
            data = self.get_event_payload()
        if headers is None:
            headers = {'content-type': 'application/x-ndjson'}
        return requests.post(url, data=data, headers=headers)


class ElasticTest(ServerBaseTest):
    skip_clean_pipelines = False

    def config(self):
        cfg = super(ElasticTest, self).config()
        cfg.update({
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
            "kibana_enabled": "false",
        })
        cfg.update(self.config_overrides)
        return cfg

    def setUp(self):
        admin_user = os.getenv("ES_SUPERUSER_USER", "admin")
        admin_password = os.getenv("ES_SUPERUSER_PASS", "changeme")
        self.admin_es = Elasticsearch([self.get_elasticsearch_url(admin_user, admin_password)])
        self.es = Elasticsearch([self.get_elasticsearch_url()])
        self.kibana = Kibana(self.get_kibana_url())

        delete_pipelines = [] if self.skip_clean_pipelines else default_pipelines
        cleanup(self.admin_es, delete_pipelines=delete_pipelines)
        self.kibana.delete_all_agent_config()

        super(ElasticTest, self).setUp()

        # try make sure APM Server is fully up
        self.wait_until_ilm_logged()
        self.wait_until_pipeline_logged()

    def wait_until_ilm_logged(self):
        setup_enabled = self.config().get("ilm_setup_enabled")
        msg = "Finished index management setup." if setup_enabled != "false" else "Manage ILM setup is disabled."
        wait_until(lambda: self.log_contains(msg), name="ILM setup")

    def wait_until_pipeline_logged(self):
        registration_enabled = self.config().get("register_pipeline_enabled")
        msg = "Registered Ingest Pipelines successfully" if registration_enabled != "false" else "No pipeline callback registered"
        wait_until(lambda: self.log_contains(msg), name="pipelines registration")

    def load_docs_with_template(self, data_path, url, endpoint, expected_events_count,
                                query_index=None, max_timeout=10, extra_headers=None, file_mode="rb"):

        if query_index is None:
            query_index = apm_prefix

        headers = {'content-type': 'application/x-ndjson'}
        if extra_headers:
            headers.update(extra_headers)

        with open(data_path, file_mode) as f:
            r = requests.post(url, data=f, headers=headers)
        assert r.status_code == 202, r.status_code
        # Wait to give documents some time to be sent to the index
        self.wait_for_events(endpoint, expected_events_count, index=query_index, max_timeout=max_timeout)

    def wait_for_events(self, processor_name, expected_count, index=None, max_timeout=10):
        """
        wait_for_events waits for an expected number of event docs with the given
        'processor.name' value, and returns the hits when found.
        """
        if index is None:
            index = apm_prefix

        query = {"term": {"processor.name": processor_name}}
        result = {}  # TODO(axw) use "nonlocal" when we migrate to Python 3

        def get_docs():
            hits = self.es.search(index=index, body={"query": query})['hits']
            result['docs'] = hits['hits']
            return hits['total']['value'] == expected_count

        wait_until(get_docs,
                   max_timeout=max_timeout,
                   name="{} documents to reach {}".format(processor_name, expected_count),
                   )
        return result['docs']

    def check_backend_error_sourcemap(self, index, count=1):
        rs = self.es.search(index=index, params={"rest_total_hits_as_int": "true"})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            for exception in err.get("exception", []):
                self.check_for_no_smap(exception)
            if "log" in err:
                self.check_for_no_smap(err["log"])

    def check_backend_span_sourcemap(self, count=1):
        rs = self.es.search(index=index_span, params={"rest_total_hits_as_int": "true"})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            self.check_for_no_smap(doc["_source"]["span"])

    def check_for_no_smap(self, doc):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            assert "sourcemap" not in frame, frame

    def logged_requests(self, url="/intake/v2/events"):
        for line in self.get_log_lines():
            jline = json.loads(line)
            u = urlparse(jline.get("URL", ""))
            if jline.get("logger") == "request" and u.path == url:
                yield jline

    def approve_docs(self, base_path, received):
        """
        approve_docs compares the received documents to those contained
        in the file at ${base_path}.approved.json. If that file does not
        exist, then it is considered equivalent to a lack of documents.

        Only the document _source is compared, and we ignore differences
        in some context-sensitive fields such as the "observer", which
        may vary between test runs.
        """
        base_path = self._beat_path_join(os.path.dirname(__file__), base_path)
        approved_path = base_path + '.approved.json'
        received_path = base_path + '.received.json'
        try:
            with open(approved_path) as f:
                approved = json.load(f)
        except IOError:
            approved = []

        # get_doc_id returns a value suitable for sorting and identifying
        # documents: either a unique ID, or a timestamp. This is necessary
        # since not all event types require a unique ID (namely, errors do
        # not.)
        #
        # We return (0, doc['error']['id']) when the event type is 'error'
        # if that field exists, otherwise returns (1, doc['@timestamp']).
        # The first tuple element exists to sort IDs before timestamps.
        def get_doc_id(doc):
            doc_type = doc['processor']['event']
            if 'id' in doc.get(doc_type, {}):
                return (0, doc[doc_type]['id'])
            if doc_type == 'metric' and 'transaction' in doc:
                transaction = doc['transaction']
                if 'histogram' in transaction.get('duration', {}):
                    # Transaction histogram documents are published periodically
                    # by the apm-server, so we cannot use the timestamp. Instead,
                    # use the transaction name, type, and result (a subset of the
                    # full aggregation key, but good enough for our tests).
                    fields = [transaction.get(field, '') for field in ('type', 'name', 'result')]
                    return (1, '_'.join(fields))
            return (2, doc['@timestamp'])

        received = [doc['_source'] for doc in received]
        received.sort(key=get_doc_id)

        try:
            for rec in received:
                # Overwrite received observer values with the approved ones,
                # in order to avoid noise in the 'approvals' diff if there are
                # any other changes.
                #
                # We don't compare the observer values between received/approved,
                # as they are dependent on the environment.
                rec_id = get_doc_id(rec)
                rec_observer = rec['observer']
                self.assertEqual(set(rec_observer.keys()), set(
                    ["hostname", "version", "id", "ephemeral_id", "type", "version_major"]))
                assert rec_observer["version"].startswith(str(rec_observer["version_major"]) + ".")
                for appr in approved:
                    if get_doc_id(appr) == rec_id:
                        rec['observer'] = appr['observer']
                        # ensure both docs have the same event keys set
                        self.assertEqual(rec.get("event", {}).keys(), appr.get("event", {}).keys())
                        # We don't compare the event values between received/approved
                        # as they are dependent on the environment.
                        if 'event' in rec:
                            rec['event'] = appr['event']
                        break
            assert len(received) == len(approved)
            for i, rec in enumerate(received):
                appr = approved[i]
                rec_id = get_doc_id(rec)
                assert rec_id == get_doc_id(appr), "New entry with id {}".format(rec_id)
                for k, v in rec.items():
                    self.assertEqual(v, appr[k])
        except Exception as exc:
            with open(received_path, 'w') as f:
                json.dump(received, f, indent=4, separators=(',', ': '), sort_keys=True)

            # Create a dynamic Exception subclass so we can fake its name to look like the original exception.
            class ApprovalException(Exception):
                def __init__(self, cause):
                    super(ApprovalException, self).__init__(cause.args)

                def __str__(self):
                    return "{}\n\nReceived data differs from approved data. Run 'make update check-approvals' to verify the diff.".format(self.args)
            ApprovalException.__name__ = type(exc).__name__
            raise ApprovalException(exc).with_traceback(sys.exc_info()[2])


class ClientSideBaseTest(ServerBaseTest):
    sourcemap_url = 'http://localhost:8200/assets/v1/sourcemaps'
    intake_url = 'http://localhost:8200/intake/v2/rum/events'
    backend_intake_url = 'http://localhost:8200/intake/v2/events'
    config_overrides = {}

    def config(self):
        cfg = super(ClientSideBaseTest, self).config()
        cfg.update({"enable_rum": "true",
                    "kibana_enabled": "false",
                    "smap_cache_expiration": "200"})
        cfg.update(self.config_overrides)
        return cfg

    def get_backend_error_payload_path(self, name="errors_2.ndjson"):
        return super(ClientSideBaseTest, self).get_payload_path(name)

    def get_backend_transaction_payload_path(self, name="transactions_spans.ndjson"):
        return super(ClientSideBaseTest, self).get_payload_path(name)

    def get_error_payload_path(self, name="errors_rum.ndjson"):
        return super(ClientSideBaseTest, self).get_payload_path(name)

    def get_transaction_payload_path(self, name="transactions_spans_rum_2.ndjson"):
        return super(ClientSideBaseTest, self).get_payload_path(name)


class ClientSideElasticTest(ClientSideBaseTest, ElasticTest):
    def check_rum_error_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=index_error, params={"rest_total_hits_as_int": "true"})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            for exception in err.get("exception", []):
                self.check_smap(exception, updated, expected_err)
            if "log" in err:
                self.check_smap(err["log"], updated, expected_err)

    def check_rum_transaction_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=index_span, params={"rest_total_hits_as_int": "true"})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            span = doc["_source"]["span"]
            self.check_smap(span, updated, expected_err)

    @staticmethod
    def check_smap(doc, updated, err=None):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            smap = frame["sourcemap"]
            if err is None:
                assert 'error' not in smap
            else:
                assert err in smap["error"]
            assert smap["updated"] == updated


class CorsBaseTest(ClientSideBaseTest):
    def config(self):
        cfg = super(CorsBaseTest, self).config()
        cfg.update({"allow_origins": ["http://www.elastic.co"]})
        return cfg


class ExpvarBaseTest(ServerBaseTest):
    config_overrides = {}

    def config(self):
        cfg = super(ExpvarBaseTest, self).config()
        cfg.update(self.config_overrides)
        return cfg

    def get_debug_vars(self):
        return requests.get(self.expvar_url)


class SubCommandTest(ServerBaseTest):
    config_overrides = {}

    def config(self):
        cfg = super(SubCommandTest, self).config()
        cfg.update({
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
        })
        cfg.update(self.config_overrides)
        return cfg

    def wait_until_started(self):
        self.apmserver_proc.check_wait()

        # command and go test output is combined in log, pull out the command output
        log = self.get_log()
        pos = -1
        for _ in range(2):
            # export always uses \n, not os.linesep
            pos = log[:pos].rfind("\n")
        self.command_output = log[:pos]
        for trimmed in log[pos:].strip().splitlines():
            # ensure only skipping expected lines
            assert trimmed.split(None, 1)[0] in ("PASS", "coverage:"), trimmed

    def stop_proc(self):
        return


class ProcStartupFailureTest(ServerBaseTest):

    def stop_proc(self):
        try:
            self.apmserver_proc.kill_and_wait()
        except:
            self.apmserver_proc.wait()

    def wait_until_started(self):
        return
