from datetime import datetime, timedelta
import json
import os
import re
import shutil
from time import gmtime, strftime
from urlparse import urlparse

import sys
import time

from elasticsearch import Elasticsearch
import requests

sys.path.append(os.path.join(os.path.dirname(__file__), '..',
                             '..', '_beats', 'libbeat', 'tests', 'system'))
from beat.beat import TestCase, TimeoutError


class BaseTest(TestCase):
    maxDiff = None

    @classmethod
    def setUpClass(cls):
        cls.apm_version = "7.5.0"
        cls.day = strftime("%Y.%m.%d", gmtime())
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
        cls.index_acm = ".apm-agent-configuration"
        cls.indices = [cls.index_onboarding, cls.index_error,
                       cls.index_transaction, cls.index_span, cls.index_metric, cls.index_smap]

        super(BaseTest, cls).setUpClass()

    @classmethod
    def _beat_path_join(cls, *paths):
        return os.path.abspath(os.path.join(cls.beat_path, *paths))

    def get_payload_path(self, name):
        return self._beat_path_join(
            'testdata',
            'intake-v2',
            name)

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


class ServerSetUpBaseTest(BaseTest):
    host = "http://localhost:8200"
    agent_config_url = "{}/{}".format(host, "config/v1/agents")
    intake_url = "{}/{}".format(host, 'intake/v2/events')
    expvar_url = "{}/{}".format(host, 'debug/vars')

    def config(self):
        return {"ssl_enabled": "false",
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

    def start_args(self):
        return {}

    def wait_until_started(self):
        self.wait_until(lambda: self.log_contains("Starting apm-server"))

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

    def request_intake(self, data="", url="", headers={'content-type': 'application/x-ndjson'}):
        if url == "":
            url = self.intake_url
        if data == "":
            data = self.get_event_payload()
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
            "kibana_host": self.get_kibana_url(),
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
        self.es.indices.delete(index="apm*", ignore=[400, 404])
        for idx in self.indices:
            self.wait_until(lambda: not self.es.indices.exists(idx))

        self.es.indices.delete_template(name="apm*", ignore=[400, 404])
        for idx in self.indices:
            self.wait_until(lambda: not self.es.indices.exists_template(idx))

        # truncate, don't delete agent configuration index since it's only created when kibana starts up
        self.es.delete_by_query(self.index_acm, {"query": {"match_all": {}}},
                                ignore_unavailable=True, wait_for_completion=True)
        self.wait_until(lambda: self.es.count(index=self.index_acm, ignore_unavailable=True)["count"] == 0)
        # Cleanup pipelines
        self.es.ingest.delete_pipeline(id="*")

        super(ElasticTest, self).setUp()

    def load_docs_with_template(self, data_path, url, endpoint, expected_events_count,
                                query_index=None, max_timeout=10):

        if query_index is None:
            query_index = self.index_name_pattern

        with open(data_path) as f:
            r = requests.post(url,
                              data=f,
                              headers={'content-type': 'application/x-ndjson'})
        assert r.status_code == 202, r.status_code

        # make sure template is loaded
        self.wait_until(
            lambda: self.log_contains("Finished loading index template"),
            max_timeout=max_timeout)
        self.wait_until(lambda: self.es.indices.exists(query_index))
        # Quick wait to give documents some time to be sent to the index
        # This is not required but speeds up the tests
        time.sleep(0.1)
        self.es.indices.refresh(index=query_index)

        self.wait_until(
            lambda: (self.es.count(index=query_index, body={
                "query": {"term": {"processor.name": endpoint}}}
            )['count'] == expected_events_count),
            max_timeout=max_timeout
        )

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
                         bundle_filepath='bundle_no_mapping.js.map',
                         expected_ct=1):
        path = self._beat_path_join(
            'testdata',
            'sourcemap',
            file_name)
        f = open(path)
        r = requests.post(self.sourcemap_url,
                          files={'sourcemap': f},
                          data={'service_version': service_version,
                                'bundle_filepath': bundle_filepath,
                                'service_name': service_name
                                })
        return r


class ClientSideElasticTest(ClientSideBaseTest, ElasticTest):
    def wait_for_sourcemaps(self, expected_ct=1):
        idx = self.index_smap
        self.wait_until(
            lambda: (self.es.count(index=idx, body={
                "query": {"term": {"processor.name": 'sourcemap'}}}
            )['count'] == expected_ct)
        )

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

    def config(self):
        cfg = super(SubCommandTest, self).config()
        cfg.update({
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
        })
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
