from datetime import datetime, timedelta
import json
import os
import re
import sets
import shutil
import sys
import threading
import time
import unittest
from urlparse import urlparse

from elasticsearch import Elasticsearch
from nose.tools import nottest
import requests

sys.path.append(os.path.join(os.path.dirname(__file__), '..',
                             '..', '_beats', 'libbeat', 'tests', 'system'))
from beat.beat import INTEGRATION_TESTS, TestCase, TimeoutError

integration_test = unittest.skipUnless(INTEGRATION_TESTS, "integration test")
diagnostic_interval = float(os.environ.get('DIAGNOSTIC_INTERVAL', 0))


class BaseTest(TestCase):
    maxDiff = None

    def setUp(self):
        super(BaseTest, self).setUp()
        if diagnostic_interval > 0:
            self.diagnostics_path = os.path.join(self.working_dir, "diagnostics")
            os.makedirs(self.diagnostics_path)
            self.running = True
            self.diagnostic_thread = threading.Thread(
                target=self.dump_diagnotics, kwargs=dict(interval=diagnostic_interval))
            self.diagnostic_thread.daemon = True
            self.diagnostic_thread.start()

    def tearDown(self):
        if diagnostic_interval > 0:
            self.running = False
            self.diagnostic_thread.join(timeout=30)
        super(BaseTest, self).tearDown()

    @classmethod
    def setUpClass(cls):
        cls.apm_version = "8.0.0"
        cls.day = time.strftime("%Y.%m.%d", time.gmtime())
        cls.beat_name = "apm-server"
        cls.beat_path = os.path.abspath(os.path.join(
            os.path.dirname(__file__), "..", ".."))
        cls.build_path = cls._beat_path_join("build", "system-tests")

        cls.index_name = "apm-{}".format(cls.apm_version)
        cls.index_name_pattern = "apm-*"
        cls.index_onboarding = "apm-{}-onboarding-{}".format(cls.apm_version, cls.day)
        cls.index_error = "apm-{}-error".format(cls.apm_version)
        cls.index_transaction = "apm-{}-transaction".format(cls.apm_version)
        cls.index_span = "apm-{}-span".format(cls.apm_version)
        cls.index_metric = "apm-{}-metric".format(cls.apm_version)
        cls.index_smap = "apm-{}-sourcemap".format(cls.apm_version)
        cls.index_profile = "apm-{}-profile".format(cls.apm_version)
        cls.index_acm = ".apm-agent-configuration"
        cls.indices = [cls.index_onboarding, cls.index_error, cls.index_transaction,
                       cls.index_span, cls.index_metric, cls.index_smap, cls.index_profile]

        super(BaseTest, cls).setUpClass()

    @classmethod
    def _beat_path_join(cls, *paths):
        return os.path.abspath(os.path.join(cls.beat_path, *paths))

    @staticmethod
    def get_elasticsearch_url(user="", password=""):
        """
        Returns an elasticsearch.Elasticsearch instance built from the
        env variables like the integration tests.
        """
        host = os.getenv("ES_HOST", "localhost")

        if not user:
            user = os.getenv("ES_USER", "admin")
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
        Returns kibana host URL
        """
        host = os.getenv("KIBANA_HOST", "localhost")

        if not user:
            user = os.getenv("KIBANA_USER", "admin")
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

    @nottest
    def get_testdata_path(self, *names):
        return self._beat_path_join('testdata', *names)

    def get_payload(self, name):
        with open(self.get_payload_path(name)) as f:
            return f.read()

    def get_error_payload_path(self):
        return self.get_payload_path("errors_2.ndjson")

    def get_transaction_payload_path(self):
        return self.get_payload_path("transactions.ndjson")

    def get_metricset_payload_payload_path(self):
        return self.get_payload_path("metricsets.ndjson")

    def get_event_payload(self, name="events.ndjson"):
        return self.get_payload(name)

    def ilm_index(self, index):
        return "{}-000001".format(index)

    def dump_diagnotics(self, interval=2):
        while self.running:
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


class ServerSetUpBaseTest(BaseTest):
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
        return {"ssl_enabled": "false",
                "queue_flush": 0,
                "jaeger_grpc_enabled": "true",
                "jaeger_grpc_host": self.jaeger_grpc_host,
                "jaeger_http_enabled": "true",
                "jaeger_http_host": self.jaeger_http_host,
                "path": os.path.abspath(self.working_dir) + "/log/*"}

    def setUp(self):
        super(ServerSetUpBaseTest, self).setUp()
        shutil.copy(self._beat_path_join("fields.yml"), self.working_dir)

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
        self.apmserver_proc = self.start_beat(**self.start_args())
        self.wait_until_started()

        # try make sure APM Server is fully up
        cfg = self.config()
        # pipeline registration is enabled by default and only happens if the output is elasticsearch
        if not getattr(self, "register_pipeline_disabled", False) and \
                cfg.get("elasticsearch_host") and \
                cfg.get("register_pipeline_enabled") != "false" and cfg.get("register_pipeline_overwrite") != "false":
            self.wait_until_pipelines_registered()

    def start_args(self):
        return {}

    def wait_until_started(self):
        self.wait_until(lambda: self.log_contains("Starting apm-server"), name="apm-server started")

    def wait_until_ilm_setup(self):
        self.wait_until(lambda: self.log_contains("Finished index management setup."), name="ILM setup")

    def wait_until_pipelines_registered(self):
        self.wait_until(lambda: self.log_contains("Registered Ingest Pipelines successfully"),
                        name="pipelines registered")

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
        self.assertNotRegexpMatches(log, "ERR|WARN")

    def request_intake(self, data=None, url=None, headers=None):
        if not url:
            url = self.intake_url
        if data is None:
            data = self.get_event_payload()
        if headers is None:
            headers = {'content-type': 'application/x-ndjson'}
        return requests.post(url, data=data, headers=headers)


class ServerBaseTest(ServerSetUpBaseTest):
    def tearDown(self):
        super(ServerBaseTest, self).tearDown()
        self.apmserver_proc.check_kill_and_wait()


class ElasticTest(ServerBaseTest):
    config_overrides = {}

    def config(self):
        cfg = super(ElasticTest, self).config()
        cfg.update({
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
            "kibana_enabled": "false",
        })
        cfg.update(self.config_overrides)
        return cfg

    def wait_until(self, cond, max_timeout=10, poll_interval=0.1, name="cond"):
        """
        Like beat.beat.wait_until but catches exceptions
        In a ElasticTest `cond` will usually be a query, and we need to keep retrying
         eg. on 503 response codes
        """
        start = datetime.now()
        result = False
        while not result:
            try:
                result = cond()
            except:
                result = False
            if datetime.now() - start > timedelta(seconds=max_timeout):
                raise TimeoutError("Timeout waiting for '{}' to be true. ".format(name) +
                                   "Waited {} seconds.".format(max_timeout))
            time.sleep(poll_interval)
        return result

    def setUp(self):
        self.es = Elasticsearch([self.get_elasticsearch_url()])
        self.kibana_url = self.get_kibana_url()

        # Cleanup index and template first
        assert all(idx.startswith("apm")
                   for idx in self.indices), "not all indices prefixed with apm, cleanup assumption broken"
        if self.es.indices.get("apm*"):
            self.es.indices.delete(index="apm*", ignore=[400, 404])
            for idx in self.indices:
                self.wait_until(lambda: not self.es.indices.exists(idx), name="index {} to be deleted".format(idx))

        if self.es.indices.get_template(name="apm*", ignore=[400, 404]):
            self.es.indices.delete_template(name="apm*", ignore=[400, 404])
            for idx in self.indices:
                self.wait_until(lambda: not self.es.indices.exists_template(idx),
                                name="index template {} to be deleted".format(idx))

        # truncate, don't delete agent configuration index since it's only created when kibana starts up
        if self.es.count(index=self.index_acm, ignore_unavailable=True)["count"] > 0:
            self.es.delete_by_query(self.index_acm, {"query": {"match_all": {}}},
                                    ignore_unavailable=True, wait_for_completion=True)
            self.wait_until(lambda: self.es.count(index=self.index_acm, ignore_unavailable=True)["count"] == 0,
                            max_timeout=30, name="acm index {} to be empty".format(self.index_acm))

        # Cleanup pipelines
        if self.es.ingest.get_pipeline(ignore=[400, 404]):
            self.es.ingest.delete_pipeline(id="*")

        super(ElasticTest, self).setUp()

    def load_docs_with_template(self, data_path, url, endpoint, expected_events_count,
                                query_index=None, max_timeout=10, extra_headers=None):

        if query_index is None:
            query_index = self.index_name_pattern

        headers = {'content-type': 'application/x-ndjson'}
        if extra_headers:
            headers.update(extra_headers)

        with open(data_path) as f:
            r = requests.post(url, data=f, headers=headers)
        assert r.status_code == 202, r.status_code

        # Wait to give documents some time to be sent to the index
        self.wait_for_events(endpoint, expected_events_count, index=query_index)

    def wait_for_events(self, processor_name, expected_count, index=None, max_timeout=10):
        """
        wait_for_events waits for an expected number of event docs with the given
        'processor.name' value, and returns the hits when found.
        """
        if index is None:
            index = self.index_name_pattern

        query = {"term": {"processor.name": processor_name}}
        result = {}  # TODO(axw) use "nonlocal" when we migrate to Python 3

        def get_docs():
            hits = self.es.search(index=index, body={"query": query})['hits']
            result['docs'] = hits['hits']
            return hits['total']['value'] == expected_count

        self.wait_until(
            get_docs,
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
        rs = self.es.search(index=self.index_span, params={"rest_total_hits_as_int": "true"})
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
            if 'id' in doc[doc_type]:
                return (0, doc[doc_type]['id'])
            return (1, doc['@timestamp'])

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
                self.assertEqual(sets.Set(rec_observer.keys()), sets.Set(
                    ["hostname", "version", "id", "ephemeral_id", "type", "version_major"]))
                assert rec_observer["version"].startswith(str(rec_observer["version_major"]) + ".")
                for appr in approved:
                    if get_doc_id(appr) == rec_id:
                        rec['observer'] = appr['observer']
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
                json.dump(received, f, indent=4, separators=(',', ': '))

            # Create a dynamic Exception subclass so we can fake its name to look like the original exception.
            class ApprovalException(Exception):
                def __init__(self, cause):
                    super(ApprovalException, self).__init__(cause.message)

                def __str__(self):
                    return self.message + "\n\nReceived data differs from approved data. Run 'make update' and then 'approvals' to verify the diff."
            ApprovalException.__name__ = type(exc).__name__
            raise ApprovalException, exc, sys.exc_info()[2]


class ClientSideBaseTest(ServerBaseTest):
    sourcemap_url = 'http://localhost:8200/assets/v1/sourcemaps'
    intake_url = 'http://localhost:8200/intake/v2/rum/events'
    backend_intake_url = 'http://localhost:8200/intake/v2/events'
    config_overrides = {}

    @classmethod
    def setUpClass(cls):
        super(ClientSideBaseTest, cls).setUpClass()

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

    def upload_sourcemap(self, file_name='bundle_no_mapping.js.map',
                         service_name='apm-agent-js',
                         service_version='1.0.1',
                         bundle_filepath='bundle_no_mapping.js.map'):
        path = self._beat_path_join('testdata', 'sourcemap', file_name)
        with open(path) as f:
            return requests.post(self.sourcemap_url,
                                 files={'sourcemap': f},
                                 data={'service_version': service_version,
                                       'bundle_filepath': bundle_filepath,
                                       'service_name': service_name})


class ClientSideElasticTest(ClientSideBaseTest, ElasticTest):
    def wait_for_sourcemaps(self, expected_ct=1):
        self.wait_for_events('sourcemap', expected_ct, index=self.index_smap)

    def check_rum_error_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=self.index_error, params={"rest_total_hits_as_int": "true"})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            for exception in err.get("exception", []):
                self.check_smap(exception, updated, expected_err)
            if "log" in err:
                self.check_smap(err["log"], updated, expected_err)

    def check_rum_transaction_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=self.index_span, params={"rest_total_hits_as_int": "true"})
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


class OverrideIndicesTest(ElasticTest):

    def config(self):
        cfg = super(OverrideIndicesTest, self).config()
        cfg.update({"override_index": self.index_name,
                    "override_template": self.index_name})
        return cfg


class OverrideIndicesFailureTest(ElasticTest):

    def config(self):
        cfg = super(OverrideIndicesFailureTest, self).config()
        cfg.update({"override_index": self.index_name, })
        return cfg

    def wait_until(self, cond, max_timeout=10, poll_interval=0.1, name="cond"):
        return

    def tearDown(self):
        return


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


class SubCommandTest(ServerSetUpBaseTest):
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
