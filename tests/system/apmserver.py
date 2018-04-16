import json
import os
import re
import shutil

import requests
import sys
import time
from elasticsearch import Elasticsearch

sys.path.append(os.path.join(os.path.dirname(__file__), '..', '..', '_beats', 'libbeat', 'tests', 'system'))
from beat.beat import TestCase


class BaseTest(TestCase):

    @classmethod
    def setUpClass(cls):
        cls.beat_name = "apm-server"
        cls.beat_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        cls.build_path = cls._beat_path_join("build", "system-tests")
        cls.index_name = "test-apm-12-12-2017"
        super(BaseTest, cls).setUpClass()

    @classmethod
    def _beat_path_join(cls, *paths):
        return os.path.abspath(os.path.join(cls.beat_path, *paths))

    def get_transaction_payload_path(self, name="payload.json"):
        return self._beat_path_join(
            'tests',
            'data',
            'valid',
            'transaction',
            name)

    def get_transaction_payload(self):
        path = self.get_transaction_payload_path()
        return json.loads(open(path).read())

    def get_error_payload_path(self, name="payload.json"):
        return self._beat_path_join(
            'tests',
            'data',
            'valid',
            'error',
            name)

    def get_error_payload(self):
        path = self.get_error_payload_path()
        return json.loads(open(path).read())


class ServerSetUpBaseTest(BaseTest):
    transactions_url = 'http://localhost:8200/v1/transactions'
    errors_url = 'http://localhost:8200/v1/errors'
    expvar_url = 'http://localhost:8200/debug/vars'

    def config(self):
        return {"ssl_enabled": "false",
                "path": os.path.abspath(self.working_dir) + "/log/*"}

    def setUp(self):
        super(ServerSetUpBaseTest, self).setUp()
        shutil.copy(self._beat_path_join("fields.yml"), self.working_dir)

        self.render_config_template(**self.config())
        self.apmserver_proc = self.start_beat()
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

        suppress = suppress + ["WARN EXPERIMENTAL", "WARN BETA", winErr]
        log = self.get_log()
        for s in suppress:
            log = re.sub(s, "", log)
        self.assertNotRegexpMatches(log, "ERR|WARN")


class ServerBaseTest(ServerSetUpBaseTest):
    def tearDown(self):
        super(ServerBaseTest, self).tearDown()
        self.apmserver_proc.check_kill_and_wait()


class SecureServerBaseTest(ServerSetUpBaseTest):

    @classmethod
    def setUpClass(cls):
        super(SecureServerBaseTest, cls).setUpClass()

        try:
            import urllib3
            urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        except ImportError:
            pass

        try:
            from requests.packages import urllib3
            urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        except ImportError:
            pass

    def config(self):
        cfg = super(SecureServerBaseTest, self).config()
        cfg.update({
            "ssl_enabled": "true",
            "ssl_cert": self._beat_path_join("tests", "system", "config", "certs", "cert.pem"),
            "ssl_key": self._beat_path_join("tests", "system", "config", "certs", "key.pem"),
        })
        return cfg

    def tearDown(self):
        super(SecureServerBaseTest, self).tearDown()
        self.apmserver_proc.kill_and_wait()


class AccessTest(ServerBaseTest):

    def config(self):
        cfg = super(AccessTest, self).config()
        cfg.update({"secret_token": "1234"})
        return cfg


class ElasticTest(ServerBaseTest):

    def config(self):
        cfg = super(ElasticTest, self).config()
        cfg.update({
            "elasticsearch_host": self.get_elasticsearch_url(),
            "file_enabled": "false",
            "index_name": self.index_name,
        })
        return cfg

    def setUp(self):
        self.es = Elasticsearch([self.get_elasticsearch_url()])

        # Cleanup index and template first
        self.es.indices.delete(index="*", ignore=[400, 404])
        self.wait_until(lambda: not self.es.indices.exists(self.index_name))

        self.es.indices.delete_template(
            name="*", ignore=[400, 404])
        self.wait_until(
            lambda: not self.es.indices.exists_template(self.index_name))

        super(ElasticTest, self).setUp()

    def get_elasticsearch_url(self):
        """
        Returns an elasticsearch.Elasticsearch url built from the
        env variables like the integration tests.
        """
        return "http://{host}:{port}".format(
            host=os.getenv("ES_HOST", "localhost"),
            port=os.getenv("ES_PORT", "9200"),
        )

    def load_docs_with_template(self, data_path, url, endpoint, expected_events_count, query_index=None):

        if query_index is None:
            query_index = self.index_name

        payload = json.loads(open(data_path).read())
        r = requests.post(url, json=payload)
        assert r.status_code == 202

        # make sure template is loaded
        self.wait_until(
            lambda: self.log_contains(
                "Elasticsearch template with name \'{}\' loaded".format(self.index_name)),
            max_timeout=20)
        self.wait_until(lambda: self.es.indices.exists(query_index))
        # Quick wait to give documents some time to be sent to the index
        # This is not required but speeds up the tests
        time.sleep(0.1)
        self.es.indices.refresh(index=query_index)

        self.wait_until(
            lambda: (self.es.count(index=query_index, body={
                "query": {"term": {"processor.name": endpoint}}}
            )['count'] == expected_events_count)
        )

    def check_backend_error_sourcemap(self, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            if "exception" in err:
                self.check_for_no_smap(err["exception"])
            if "log" in err:
                self.check_for_no_smap(err["log"])

    def check_backend_transaction_sourcemap(self, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "span"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            self.check_for_no_smap(doc["_source"]["span"])

    def check_for_no_smap(self, doc):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            assert "sourcemap" not in frame


class ClientSideBaseTest(ElasticTest):

    transactions_url = 'http://localhost:8200/v1/client-side/transactions'
    errors_url = 'http://localhost:8200/v1/client-side/errors'
    sourcemap_url = 'http://localhost:8200/v1/client-side/sourcemaps'

    @classmethod
    def setUpClass(cls):
        super(ClientSideBaseTest, cls).setUpClass()
        cls.smap_index_pattern = "test-apm*"

    def config(self):
        cfg = super(ClientSideBaseTest, self).config()
        cfg.update({"enable_frontend": "true",
                    "smap_index_pattern": self.smap_index_pattern,
                    "smap_cache_expiration": "200"})
        return cfg

    def get_transaction_payload_path(self, name='frontend.json'):
        return super(ClientSideBaseTest, self).get_transaction_payload_path(name)

    def get_error_payload_path(self, name='frontend.json'):
        return super(ClientSideBaseTest, self).get_error_payload_path(name)

    def upload_sourcemap(self, file_name='bundle_no_mapping.js.map',
                         service_name='apm-agent-js',
                         service_version='1.0.1',
                         bundle_filepath='bundle_no_mapping.js.map',
                         expected_ct=1):
        path = self._beat_path_join(
            'tests',
            'data',
            'valid',
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

    def wait_for_sourcemaps(self, expected_ct=1):
        idx = self.smap_index_pattern
        self.wait_until(
            lambda: (self.es.count(index=idx, body={
                "query": {"term": {"processor.name": 'sourcemap'}}}
            )['count'] == expected_ct)
        )

    def check_frontend_error_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "error"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            err = doc["_source"]["error"]
            if "exception" in err:
                self.check_smap(err["exception"], updated, expected_err)
            if "log" in err:
                self.check_smap(err["log"], updated, expected_err)

    def check_frontend_transaction_sourcemap(self, updated, expected_err=None, count=1):
        rs = self.es.search(index=self.index_name, body={
            "query": {"term": {"processor.event": "span"}}})
        assert rs['hits']['total'] == count, "found {} documents, expected {}".format(
            rs['hits']['total'], count)
        for doc in rs['hits']['hits']:
            span = doc["_source"]["span"]
            self.check_smap(span, updated, expected_err)

    def check_smap(self, doc, updated, err=None):
        if "stacktrace" not in doc:
            return
        for frame in doc["stacktrace"]:
            smap = frame["sourcemap"]
            if err is None:
                assert 'error' not in smap
            else:
                assert err in smap["error"]
            assert smap["updated"] == updated


class SplitIndicesTest(ElasticTest):

    @classmethod
    def setUpClass(cls):
        super(SplitIndicesTest, cls).setUpClass()
        cls.index_name_transaction = "test-apm-transaction-12-12-2017"
        cls.index_name_span = "test-apm-span-12-12-2017"
        cls.index_name_error = "test-apm-error-12-12-2017"

    def config(self):
        cfg = super(SplitIndicesTest, self).config()
        cfg.update({"index_name_transaction": self.index_name_transaction,
                    "index_name_span": self.index_name_span,
                    "index_name_error": self.index_name_error})
        return cfg


class CorsBaseTest(ClientSideBaseTest):

    def config(self):
        cfg = super(CorsBaseTest, self).config()
        cfg.update({"allow_origins": ["http://www.elastic.co"]})
        return cfg


class SmapCacheBaseTest(ClientSideBaseTest):
    def config(self):
        cfg = super(SmapCacheBaseTest, self).config()
        cfg.update({"smap_cache_expiration": "1"})
        return cfg


class SmapIndexBaseTest(ClientSideBaseTest):
    @classmethod
    def setUpClass(cls):
        super(SmapIndexBaseTest, cls).setUpClass()
        cls.index_name = "apm-test"
        cls.smap_index_pattern = "apm-*-sourcemap*"
        cls.smap_index = "apm-test-sourcemap"

    def config(self):
        cfg = super(SmapIndexBaseTest, self).config()
        cfg.update({"index_name": "apm-test",
                    "smap_index_pattern": "apm-*-sourcemap*",
                    "smap_index": "apm-test-sourcemap"})
        return cfg


class ExpvarBaseTest(ServerBaseTest):
    config_overrides = {}

    def config(self):
        cfg = super(ExpvarBaseTest, self).config()
        cfg.update(self.config_overrides)
        return cfg

    def get_debug_vars(self):
        return requests.get(self.expvar_url)
